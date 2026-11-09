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

// Local VF and LWC host routes exercise the named Apex observations in
// reference_only_surfaces/org.tsv:79, :89 and :99. Salesforce VF/LWC hosts
// were not captured by those rows.
func TestReferenceOnlyWinterHostRoutes(t *testing.T) {
	const controller = `public class WinterReferenceHost {
 @AuraEnabled(cacheable=true) public static String run() {
  if (Type.forName('FieldServiceMobileExtension') != null) return 'reflection-failed';
  try { List<SObject> rows = [SELECT Id FROM FieldServiceMobileExtension]; return 'static-failed'; }
  catch (QueryException e) { if (e.getMessage() != 'sObject type \'FieldServiceMobileExtension\' is not supported.') return 'static-message-failed'; }
  try { List<SObject> rows = Database.query('SELECT Id FROM FieldServiceMobileExtension'); return 'dynamic-failed'; }
  catch (QueryException e) { if (e.getMessage() != 'sObject type \'FieldServiceMobileExtension\' is not supported.') return 'dynamic-message-failed'; }
  return 'ok';
 }
 public String getOutcome(){return run();}
}`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			pageAPI := "59.0"
			if api == "67.0" {
				pageAPI = "67.0"
			}
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/WinterReferenceHost.cls"), controller)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/WinterReferenceHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/WinterReferenceHost.page"), `<apex:page controller="WinterReferenceHost" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/WinterReferenceHost.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+pageAPI+`</apiVersion><label>Reference host</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			sc, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			handler := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, handler)
			handler.SetProjectIndex(typesys.Build(p, sc))
			if handler.runtimeErr != nil {
				t.Fatal(handler.runtimeErr)
			}
			for _, route := range []struct{ name, method, path, body string }{{"VF", http.MethodGet, "/apex/WinterReferenceHost", ""}, {"LWC_wire", http.MethodPost, "/lightning/wire/apex", `{"className":"WinterReferenceHost","method":"run","params":{}}`}, {"LWC_imperative", http.MethodPost, "/lightning/apex/WinterReferenceHost/run", `{}`}} {
				t.Run(route.name, func(t *testing.T) {
					for _, state := range []string{"first", "warm"} {
						t.Run(state, func(t *testing.T) {
							rec := httptest.NewRecorder()
							req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
							req.Header.Set("Content-Type", "application/json")
							handler.ServeHTTP(rec, req)
							if rec.Code != http.StatusOK {
								t.Fatalf("status=%d body=%s", rec.Code, rec.Body.String())
							}
							if route.method == http.MethodGet {
								if !strings.Contains(rec.Body.String(), ">ok<") {
									t.Fatalf("VF: %s", rec.Body.String())
								}
							} else {
								var out lwcbrowser.WireResponse
								if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
									t.Fatal(err)
								}
								if out.Error != nil || out.Data != "ok" {
									t.Fatalf("Apex response: %+v", out)
								}
							}
						})
					}
				})
			}
		})
	}
}
