package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Server route twins for aura_overload_conversion/org.tsv:2-25.
func TestLightningHistoricalOverloadProjectRoutes(t *testing.T) {
	for _, api := range []string{"45.0", "54.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), `{"sourceApiVersion":"67.0","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/classes/LegacyController.cls"), `public class LegacyController {
				@AuraEnabled public static String load(String name) { return 'one:' + name; }
				@AuraEnabled public static String load(String name, Integer count) { return 'two:' + name + ':' + String.valueOf(count); }
			}`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/classes/LegacyController.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
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
			for _, tc := range []struct {
				name, params, want string
				invalid            bool
			}{
				{"missing", `{"name":"A"}`, "two:A:null", false},
				{"null", `{"name":"A","count":null}`, "two:A:null", false},
				{"valid", `{"name":"A","count":42}`, "two:A:42", false},
				{"invalid", `{"name":"A","count":"bad"}`, "", true},
				{"object", `{"name":"A","count":{}}`, "", true},
			} {
				for _, route := range []struct{ name, path, body string }{
					{"wire", "/lightning/wire/apex", `{"className":"LegacyController","method":"load","params":` + tc.params + `}`},
					{"imperative", "/lightning/apex/LegacyController/load", tc.params},
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
						if tc.invalid {
							want := "Value provided is invalid for action parameter 'count' of type 'Integer'"
							if out.Error == nil || out.Error.Type != "InvalidActionParameter" || out.Error.Message != want {
								t.Fatalf("result=%+v want invalid parameter %q", out, want)
							}
						} else if out.Error != nil || out.Data != tc.want {
							t.Fatalf("result=%+v want %q", out, tc.want)
						}
					})
				}
			}
		})
	}
}
