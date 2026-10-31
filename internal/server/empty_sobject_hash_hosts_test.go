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

// Supplemental hosted-route tests use the anonymous and test observations in
// empty_sobject_hash/org.tsv H02 and adjacent_empty_sobject_hash/org.tsv H01.
// API 68 expectations: sobject_hash_version/org.tsv:6-7.
func TestEmptySObjectHashHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="AjaxController"></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/AjaxController.cls"), `public class AjaxController { @AuraEnabled @RemoteAction public static String hash() { return String.valueOf(System.hashCode(new Account())) + ':' + String.valueOf(System.hashCode(new Contact())) + ':' + String.valueOf(System.hashCode(new Opportunity())) + ':' + String.valueOf(System.hashCode(new Lead())) + ':' + String.valueOf(System.hashCode(new Case())); } }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/AjaxController.cls-meta.xml"), `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
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
			srv := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, srv)
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			t.Run("vf", func(t *testing.T) {
				responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"hash","data":[],"type":"rpc","tid":3}]`)
				if len(responses) != 1 || !responses[0].Status {
					t.Fatalf("remoting: %+v", responses)
				}
				value, err := json.Marshal(responses[0].Result)
				if err != nil || string(value) != `"0:0:0:0:0"` {
					t.Fatalf("remoting result %s: %v", value, err)
				}
			})
			t.Run("lwc", func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"AjaxController","method":"hash","params":{}}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				if rec.Code != http.StatusOK {
					t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
				}
				var body struct {
					Data  any `json:"data"`
					Error any `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Error != nil || body.Data != "0:0:0:0:0" {
					t.Fatalf("wire: %s, %v", rec.Body.String(), err)
				}
			})
		})
	}
}
