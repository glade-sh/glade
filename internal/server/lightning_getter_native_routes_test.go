package server

import (
	"encoding/csv"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native imperative Lightning Out observations: lwc_getter_routes/org.tsv:2-21.
// Wire requests exercise the corresponding local server route.
func TestLightningGetterNativeProjectRoutes(t *testing.T) {
	const fixture = "../apextest/testdata/conformance/lwc_getter_routes"
	file, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	reader := csv.NewReader(file)
	reader.Comma = '\t'
	rows, err := reader.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 21 {
		t.Fatalf("got %d native rows including header, want 21", len(rows))
	}

	for _, api := range []string{"62", "67"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			for _, prefix := range []string{"WireCapture", "WireDirect", "WireFinite", "WireRecursive"} {
				for _, suffix := range []string{".cls", ".cls-meta.xml"} {
					name := prefix + api + suffix
					content, err := os.ReadFile(filepath.Join(fixture, "project/force-app/main/default/classes", name))
					if err != nil {
						t.Fatal(err)
					}
					writeLightningFixtureFile(t, filepath.Join(root, "force-app/main/default/classes", name), string(content))
				}
			}
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			handler := newLightningTestServer(t, &org, source)
			handler.SetProjectIndex(typesys.Build(p, schema))
			class := "WireCapture" + api
			for _, tc := range []struct {
				name string
				want any
			}{
				{"control", "ok"},
				{"direct", map[string]any{"value": "direct", "visits": float64(1)}},
				{"finite", map[string]any{"value": "finite", "visits": float64(2)}},
				{"recursive", nil},
				{"reject", nil},
			} {
				for _, route := range []struct{ name, path, body string }{
					{"wire", "/lightning/wire/apex", `{"className":"` + class + `","method":"` + tc.name + `","params":{}}`},
					{"imperative", "/lightning/apex/" + class + "/" + tc.name, `{}`},
				} {
					t.Run(tc.name+"/"+route.name, func(t *testing.T) {
						rec := httptest.NewRecorder()
						handler.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body)))
						var out lwcbrowser.WireResponse
						if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
							t.Fatal(err)
						}
						if rec.Code != http.StatusOK {
							t.Fatalf("status=%d result=%+v", rec.Code, out)
						}
						switch tc.name {
						case "recursive":
							// Salesforce returned an internal error with a variable ID.
							if out.Error == nil {
								t.Fatalf("expected recursive action error, result=%+v", out)
							}
							if route.name == "imperative" && !strings.HasPrefix(out.Error.Message, "An internal server error has occurred") {
								t.Fatalf("expected captured internal error prefix, result=%+v", out)
							}
						case "reject":
							if out.Error == nil || out.Error.Message != "control reject" {
								t.Fatalf("expected captured rejection, result=%+v", out)
							}
						default:
							if out.Error != nil || !reflect.DeepEqual(out.Data, tc.want) {
								t.Fatalf("result=%+v want=%#v", out, tc.want)
							}
						}
					})
				}
			}
		})
	}
}
