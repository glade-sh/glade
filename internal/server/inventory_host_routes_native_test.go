package server

import (
	"encoding/json"
	"html"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// The native REST and Apex routes agree on the full profile inventory and
// stable email metadata in inventory_host_routes/org.tsv, API 62 and 67.
func TestInventoryHostRoutesNative(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "inventory_host_routes")
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var want any
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				row := strings.SplitN(line, "\t", 3)
				if len(row) == 3 && row[0] == api && row[1] == "rest" {
					if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
						t.Fatal(err)
					}
				}
			}
			if want == nil {
				t.Fatal("missing native REST observation")
			}
			root := t.TempDir()
			src := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(src, func(path string, e fs.DirEntry, err error) error {
				if err != nil || e.IsDir() {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				rel, err := filepath.Rel(src, path)
				if err != nil {
					return err
				}
				writeServerTestFile(t, filepath.Join(root, rel), strings.ReplaceAll(string(data), "capture.user@example.invalid", "system@example.invalid"))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			writeServerTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer"}`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			sch, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			idx := typesys.Build(p, sch)
			if a := sema.Analyze(idx); idx.HasErrors() || a.HasErrors() {
				t.Fatalf("project diagnostics: %v %v", idx.Diagnostics, a.Diagnostics)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			storage.EnsureDeterministicPlatformData(&org)
			storage.ApplyProjectOrgShape(&org, project.StandardFieldPolicyFeatures(p))
			if err := resource.ApplyProject(&org, p); err != nil {
				t.Fatal(err)
			}
			host := newLightningTestServer(t, &org, source)
			host.VisualforceHTMLUserID = storage.DefaultUserID
			host.SetProjectIndex(idx)
			version := strings.TrimSuffix(api, ".0")
			routes := []struct{ name, method, path, body string }{
				{"vf", http.MethodGet, "/apex/InventoryHostPageV" + version, ""},
				{"rest", http.MethodGet, "/services/apexrest/inventory-host-" + version + "/", ""},
				{"lwc-wire", http.MethodPost, "/lightning/wire/apex", `{"className":"InventoryHostV` + version + `","method":"observe","params":{}}`},
				{"lwc-imperative", http.MethodPost, "/lightning/apex/InventoryHostV" + version + "/observe", `{}`},
			}
			for _, route := range routes {
				t.Run(route.name, func(t *testing.T) {
					for _, phase := range []string{"cold", "warm"} {
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
						req.Header.Set("Content-Type", "application/json")
						host.ServeHTTP(rec, req)
						if rec.Code != http.StatusOK {
							t.Fatalf("%s HTTP %d: %s", phase, rec.Code, rec.Body.String())
						}
						var payload string
						switch route.name {
						case "vf":
							page := html.UnescapeString(rec.Body.String())
							found := false
							for i, c := range page {
								if c != '{' {
									continue
								}
								var candidate any
								if json.NewDecoder(strings.NewReader(page[i:])).Decode(&candidate) == nil && reflect.DeepEqual(candidate, want) {
									found = true
									break
								}
							}
							if !found {
								t.Fatalf("%s: page did not render native inventory", phase)
							}
							continue
						case "rest":
							if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
								t.Fatal(err)
							}
						default:
							var result lwcbrowser.WireResponse
							if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
								t.Fatal(err)
							}
							if result.Error != nil {
								t.Fatalf("%s: %+v", phase, result.Error)
							}
							var ok bool
							payload, ok = result.Data.(string)
							if !ok {
								t.Fatalf("%s non-string result: %+v", phase, result.Data)
							}
						}
						var got any
						if err := json.Unmarshal([]byte(payload), &got); err != nil {
							t.Fatal(err)
						}
						if !reflect.DeepEqual(got, want) {
							t.Fatalf("%s native=%v; Glade=%v", phase, want, got)
						}
					}
				})
			}
		})
	}
}
