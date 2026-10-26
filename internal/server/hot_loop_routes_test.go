package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Exercise the HTTP hosts that invoke the same string, XML, and list loops.
// Assertions preserve owned baseline outcomes, without new native semantics.
func TestHotLoopVisualforceAndLWCHostRoutes(t *testing.T) {
	controller, err := os.ReadFile(filepath.Join("..", "apextest", "testdata", "hot_loop_routes", "LoopShapes.cls"))
	if err != nil {
		t.Fatal(err)
	}
	const want = "300:100:7:177,154,131,108:Line: 1 Column: 7;Line: 1 Column: 9;Line: 1 Column: 12;Line: 1 Column: 12;Line: 1 Column: 17;Line: 1 Column: 23;;"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/LoopShapes.cls"), string(controller))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/LoopShapes.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/LoopHost.page"), `<apex:page controller="LoopShapes" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/LoopHost.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><label>Loop host</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			handler := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, handler)
			handler.SetProjectIndex(typesys.Build(p, schema))
			if handler.runtimeErr != nil {
				t.Fatal(handler.runtimeErr)
			}
			for _, route := range []struct{ name, method, path, body string }{
				{"VF_host", http.MethodGet, "/apex/LoopHost", ""},
				{"LWC_wire_host", http.MethodPost, "/lightning/wire/apex", `{"className":"LoopShapes","method":"run","params":{}}`},
				{"LWC_imperative_host", http.MethodPost, "/lightning/apex/LoopShapes/run", `{}`},
			} {
				t.Run(route.name, func(t *testing.T) {
					for _, state := range []string{"first_request", "warm_request"} {
						t.Run(state, func(t *testing.T) {
							rec := httptest.NewRecorder()
							req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
							req.Header.Set("Content-Type", "application/json")
							handler.ServeHTTP(rec, req)
							if rec.Code != http.StatusOK {
								t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
							}
							if route.method == http.MethodGet {
								if !strings.Contains(rec.Body.String(), ">"+want+"<") {
									t.Fatalf("missing VF outcome: %s", rec.Body.String())
								}
							} else {
								var out lwcbrowser.WireResponse
								if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
									t.Fatal(err)
								}
								if out.Error != nil || out.Data != want {
									t.Fatalf("LWC response: %+v", out)
								}
							}
							t.Log("ROW " + api + "/" + route.name + "/" + state + " outcome=" + want)
						})
					}
				})
			}
		})
	}
}
