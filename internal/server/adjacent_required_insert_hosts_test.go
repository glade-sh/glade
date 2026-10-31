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

// Supplemental host routes for D1/D2/D4/D5, insert_event_child/org.tsv:2-5.
func TestAdjacentRequiredInsertHostRoutes(t *testing.T) {
	want := "Insert failed. First exception on row 2; first error: REQUIRED_FIELD_MISSING, Required fields are missing: [LastName]: [LastName]|Insert failed. First exception on row 2; first error: REQUIRED_FIELD_MISSING, Required fields are missing: [Name]: [Name]"
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="AjaxController"></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/AjaxController.cls"), `public class AjaxController { @AuraEnabled @RemoteAction public static String fail() { String a; String b; try { Database.insert(new List<Contact>{new Contact(LastName='One'),new Contact(LastName='Two'),new Contact()},true); } catch(DmlException e) { a=e.getMessage(); } try { Database.insert(new List<Opportunity>{new Opportunity(Name='One',StageName='Prospecting',CloseDate=Date.today()),new Opportunity(Name='Two',StageName='Prospecting',CloseDate=Date.today()),new Opportunity(StageName='Prospecting',CloseDate=Date.today())},true,AccessLevel.USER_MODE); } catch(DmlException e) { b=e.getMessage(); } return a+'|'+b; } }`)
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
			storage.EnsureDeterministicPlatformData(&org)
			srv := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, srv)
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			t.Run("vf", func(t *testing.T) {
				responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"AjaxController","method":"fail","data":[],"type":"rpc","tid":3}]`)
				if len(responses) != 1 || !responses[0].Status || responses[0].Result != want {
					t.Fatalf("remoting: %+v", responses)
				}
			})
			t.Run("lwc", func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"AjaxController","method":"fail","params":{}}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				var body struct {
					Data  any `json:"data"`
					Error any `json:"error"`
				}
				if rec.Code != http.StatusOK || json.Unmarshal(rec.Body.Bytes(), &body) != nil || body.Error != nil || body.Data != want {
					t.Fatalf("wire: %s", rec.Body.String())
				}
			})
		})
	}
}
