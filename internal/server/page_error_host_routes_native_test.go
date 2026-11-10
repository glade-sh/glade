package server

import (
	"encoding/json"
	"fmt"
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
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/visualforce"
)

// Native anonymous, named-test and REST controls for missing page content
// are retained at page_error_host_routes/org.tsv, APIs 62 and 67.
func TestPageErrorHostRoutesNative(t *testing.T) {
	fixture := filepath.Join("..", "apextest", "testdata", "conformance", "page_error_host_routes")
	raw, err := os.ReadFile(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			expected := map[string]struct {
				HTTP int `json:"http"`
				Body any `json:"body"`
			}{}
			for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
				row := strings.SplitN(line, "\t", 4)
				if len(row) == 4 && row[0] == api && row[1] == "rest" {
					var v struct {
						HTTP int `json:"http"`
						Body any `json:"body"`
					}
					if err := json.Unmarshal([]byte(row[3]), &v); err != nil {
						t.Fatal(err)
					}
					expected[row[2]] = v
				}
			}
			if len(expected) != 2 {
				t.Fatal("missing native REST observations")
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
			host := newLightningTestServer(t, &org, source)
			host.VisualforceHTMLUserID = storage.DefaultUserID
			host.SetProjectIndex(idx)
			baselineEnvironments := visualforce.VMRenderEnvironmentCountForTest()
			version := strings.TrimSuffix(api, ".0")
			for _, kind := range []string{"html", "pdf"} {
				t.Run(kind, func(t *testing.T) {
					t.Run("tooling-anonymous", func(t *testing.T) {
						source, err := os.ReadFile(filepath.Join(fixture, "anon", "pagehost-anon-"+version+"-"+kind+".apex"))
						if err != nil {
							t.Fatal(err)
						}
						body, err := json.Marshal(map[string]string{"anonymousBody": string(source)})
						if err != nil {
							t.Fatal(err)
						}
						for _, phase := range []string{"cold", "warm"} {
							rec := httptest.NewRecorder()
							// The local REST endpoint exposes v67; the invoked source class
							// remains pinned to the captured Apex API 62 or 67.
							req := httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", strings.NewReader(string(body)))
							req.Header.Set("Content-Type", "application/json")
							host.ServeHTTP(rec, req)
							if got := visualforce.VMRenderEnvironmentCountForTest(); got != baselineEnvironments {
								t.Fatalf("%s leaked render environments: got %d want %d", phase, got, baselineEnvironments)
							}
							var got map[string]any
							if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &got) != nil {
								t.Fatalf("%s tooling HTTP %d: %s", phase, rec.Code, rec.Body.String())
							}
							if kind == "html" {
								if got["success"] != true || !strings.Contains(fmt.Sprint(got["logs"]), "CAPTURE|"+expected[kind].Body.(string)) {
									t.Fatalf("%s tooling HTML: %v", phase, got)
								}
							} else if got["success"] != false || !strings.Contains(fmt.Sprint(got["exceptionMessage"]), "Page MissingHostPage does not exist") {
								t.Fatalf("%s tooling PDF: %v", phase, got)
							}
						}
					})
					title := strings.ToUpper(kind[:1]) + kind[1:]
					routes := []struct{ name, method, path, body string }{
						{"vf", http.MethodGet, "/apex/PageError" + title + "V" + version, ""},
						{"rest", http.MethodGet, "/services/apexrest/page-error-" + kind + "-" + version + "/", ""},
						{"lwc-wire", http.MethodPost, "/lightning/wire/apex", `{"className":"PageErrorHostV` + version + `","method":"` + kind + `","params":{}}`},
						{"lwc-imperative", http.MethodPost, "/lightning/apex/PageErrorHostV" + version + "/" + kind, `{}`},
					}
					for _, route := range routes {
						t.Run(route.name, func(t *testing.T) {
							for _, phase := range []string{"cold", "warm"} {
								rec := httptest.NewRecorder()
								req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
								req.Header.Set("Content-Type", "application/json")
								host.ServeHTTP(rec, req)
								if got := visualforce.VMRenderEnvironmentCountForTest(); got != baselineEnvironments {
									t.Fatalf("%s leaked render environments: got %d want %d", phase, got, baselineEnvironments)
								}
								if route.name == "rest" {
									var got any
									if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
										t.Fatalf("%s: %v; body=%s", phase, err, rec.Body.String())
									}
									want := expected[kind]
									if rec.Code != want.HTTP || !reflect.DeepEqual(got, want.Body) {
										t.Fatalf("%s native HTTP %d body=%v; Glade HTTP %d body=%v", phase, want.HTTP, want.Body, rec.Code, got)
									}
									continue
								}
								if route.name == "vf" {
									if kind == "html" {
										want := expected[kind].Body.(string)
										if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), want) {
											t.Fatalf("%s VF HTML HTTP %d: %s", phase, rec.Code, rec.Body.String())
										}
									} else if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "Visualforce render error") || !strings.Contains(rec.Body.String(), "Page MissingHostPage does not exist") {
										// Local VF retains its existing HTTP 200 error-page contract.
										// Native VF status was not captured; fault text comes from
										// the independent anonymous and REST observations.
										t.Fatalf("%s VF PDF HTTP %d: %s", phase, rec.Code, rec.Body.String())
									}
									continue
								}
								var result lwcbrowser.WireResponse
								if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil {
									t.Fatalf("%s: %v; body=%s", phase, err, rec.Body.String())
								}
								if kind == "html" {
									if result.Error != nil || result.Data != expected[kind].Body {
										t.Fatalf("%s native=%v; Glade=%+v", phase, expected[kind].Body, result)
									}
								} else if result.Error == nil || !strings.Contains(result.Error.Message, "Page MissingHostPage does not exist") {
									t.Fatalf("%s PDF error missing: %+v", phase, result)
								}
							}
						})
					}
				})
			}
		})
	}
}
