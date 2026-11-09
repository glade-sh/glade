package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// P0-expanded/org.tsv records the named Apex field boundary.
func TestStandardDescribeAPI68HostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeServerTestFile(t, filepath.Join(root, "config/project-scratch-def.json"), `{"edition":"Developer"}`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="V035AjaxController"></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/V035AjaxController.cls"), `public class V035AjaxController { @AuraEnabled @RemoteAction public static Boolean observe() { return Schema.getGlobalDescribe().get('AssetStatePeriod').getDescribe().fields.getMap().containsKey('RampUpliftType'); } }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/V035AjaxController.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
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
			storage.ApplyProjectOrgShape(&org, []string{project.StandardFieldEditionDeveloper})
			storage.EnsureStandardObject(&org, "AssetStatePeriod")
			srv := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, srv)
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			want := api == "68.0"
			t.Run("vf", func(t *testing.T) {
				responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"V035AjaxController","method":"observe","data":[],"type":"rpc","tid":3}]`)
				if len(responses) != 1 || !responses[0].Status || responses[0].Result != want {
					t.Fatalf("native named result=%t, remoting=%+v", want, responses)
				}
			})
			t.Run("lwc", func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"V035AjaxController","method":"observe","params":{}}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				var body struct {
					Data  any `json:"data"`
					Error any `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK || body.Error != nil || body.Data != want {
					t.Fatalf("native named result=%t, wire=%s: %v", want, rec.Body.String(), err)
				}
			})
		})
	}
}
