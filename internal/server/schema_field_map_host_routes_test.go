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

// Exercises captured C02, C13, C35, C40, C54-C60 and alias A01/A05 through Apex hosts.
func TestSchemaFieldMapHostRoutes(t *testing.T) {
	const controller = `public class FieldMapHost {
 @AuraEnabled(cacheable=true) public static String run() {
  Map<String,Schema.SObjectField> d=Schema.SObjectType.Account.fields.getMap();
  Map<String,Schema.SObjectField> c=new Map<String,Schema.SObjectField>(d);
  System.assertEquals(Account.Name,c.get('NAME'));
  c.put('AddedKey',Contact.Name); System.assertEquals(null,c.get('AddedKey'));
  c.put('addedkey',Contact.Name); System.assertEquals(Contact.Name,c.get('ADDEDKEY'));
  c.put('NAME',Contact.Name); c.remove('name'); System.assertEquals(null,c.get('NAME'));
  Map<String,Schema.SObjectField> alias=d; Map<String,Schema.SObjectField> other=Schema.SObjectType.Account.fields.getMap();
  d.clear(); System.assertEquals(true,alias.isEmpty()); System.assertEquals(true,other.containsKey('Name'));
  c=new Map<String,Schema.SObjectField>(d); Map<String,Schema.SObjectField> l=d.clone();
  System.assertEquals(0,c.size()); System.assertEquals(0,l.size());
  return 'ok';
 }
 public String getOutcome(){return run();}
}`
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/FieldMapHost.cls"), controller)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/FieldMapHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/FieldMapHost.page"), `<apex:page controller="FieldMapHost" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/FieldMapHost.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><label>Field map host</label></ApexPage>`)
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
			for _, route := range []struct{ name, method, path, body string }{{"VF", http.MethodGet, "/apex/FieldMapHost", ""}, {"LWC_wire", http.MethodPost, "/lightning/wire/apex", `{"className":"FieldMapHost","method":"run","params":{}}`}, {"LWC_imperative", http.MethodPost, "/lightning/apex/FieldMapHost/run", `{}`}} {
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
