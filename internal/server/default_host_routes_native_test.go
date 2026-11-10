package server

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// The native anonymous, named test and Apex REST controls agree on the same
// defaults and strip result at APIs 62 and 67 (default_host_routes/org.tsv).
func TestLoadedDefaultsHostRoutesNative(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "default_host_routes")
	body, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			var want string
			for _, line := range strings.Split(strings.TrimSpace(string(body)), "\n")[1:] {
				row := strings.SplitN(line, "\t", 3)
				if len(row) == 3 && row[0] == api && row[1] == "rest" {
					if err := json.Unmarshal([]byte(row[2]), &want); err != nil {
						t.Fatal(err)
					}
				}
			}
			if want == "" {
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
				writeServerTestFile(t, filepath.Join(root, rel), string(data))
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
			host := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, host)
			profiles := host.Org.Objects["Profile"]
			profile := profiles.Records[visualforceTestProfileID]
			profile.Fields["Name"] = storage.StringValue("System Administrator")
			profiles.Records[visualforceTestProfileID] = profile
			host.Org.Objects["Profile"] = profiles
			host.SetProjectIndex(idx)
			version := strings.TrimSuffix(api, ".0")
			routes := []struct{ name, method, path, body string }{
				{"vf", http.MethodGet, "/apex/DefaultHostPageV" + version, ""},
				{"rest", http.MethodGet, "/services/apexrest/default-host-" + version + "/", ""},
				{"lwc-wire", http.MethodPost, "/lightning/wire/apex", `{"className":"DefaultHostV` + version + `","method":"observe","params":{}}`},
				{"lwc-imperative", http.MethodPost, "/lightning/apex/DefaultHostV" + version + "/observe", `{}`},
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
						if route.name == "vf" || route.name == "rest" {
							if !strings.Contains(rec.Body.String(), want) {
								t.Fatalf("%s native=%q response=%q", phase, want, rec.Body.String())
							}
							continue
						}
						var got lwcbrowser.WireResponse
						if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
							t.Fatal(err)
						}
						if got.Error != nil || got.Data != want {
							t.Fatalf("%s native=%q response=%+v", phase, want, got)
						}
					}
				})
			}
		})
	}
}
