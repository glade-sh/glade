package server

import (
	"bytes"
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

// These are local transport checks of the anonymous and isTest rows in
// namespaced_field_keyset/describe/org.tsv, not native host captures.
func TestNamespacedFieldMapVisualforceLightningAndToolingHosts(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"namespace":"garden","packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="FieldMapHost"></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>59.0</apiVersion><label>Field Map Host</label></ApexPage>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/objects/Widget__c/Widget__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Widget</label><pluralLabel>Widgets</pluralLabel><nameField><type>Text</type><label>Widget Name</label></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/objects/Widget__c/fields/Alpha__c.field-meta.xml"), `<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>Alpha__c</fullName><label>Alpha</label><length>40</length><type>Text</type></CustomField>`)
			const probe = `Map<String, Schema.SObjectField> fields = Widget__c.SObjectType.getDescribe().fields.getMap();
System.assertEquals(true, fields.containsKey('garden__Alpha__c'));
System.assertEquals(false, fields.containsKey('Alpha__c'));
System.assertEquals(true, fields.containsKey('Name'));`
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/FieldMapHost.cls"), `public class FieldMapHost { @AuraEnabled @RemoteAction public static String observe() { `+probe+` return 'ok'; } }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/FieldMapHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
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
			org.Namespace, org.APIVersion = "garden", api
			org.Objects["garden__Widget__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "garden__Widget__c", Fields: map[string]storage.Field{
				"garden__Alpha__c": {APIName: "garden__Alpha__c", Type: storage.FieldString, DisplayType: "STRING", Length: 40},
			}}, Records: map[storage.ID]storage.Record{}}
			srv := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, srv)
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			t.Run("visualforce", func(t *testing.T) {
				responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"FieldMapHost","method":"observe","data":[],"type":"rpc","tid":3}]`)
				if len(responses) != 1 || !responses[0].Status || responses[0].Result != "ok" {
					t.Fatalf("remoting: %+v", responses)
				}
			})
			t.Run("lightning", func(t *testing.T) {
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"FieldMapHost","method":"observe","params":{}}`))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				var body struct {
					Data  any `json:"data"`
					Error any `json:"error"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK || body.Error != nil || body.Data != "ok" {
					t.Fatalf("wire: %d %s, error %v", rec.Code, rec.Body.String(), err)
				}
			})
			t.Run("tooling", func(t *testing.T) {
				body, err := json.Marshal(map[string]string{"anonymousBody": `System.assertEquals('ok', FieldMapHost.observe());`})
				if err != nil {
					t.Fatal(err)
				}
				rec := httptest.NewRecorder()
				req := httptest.NewRequest(http.MethodPost, "/services/data/v67.0/tooling/executeAnonymous", bytes.NewReader(body))
				req.Header.Set("Content-Type", "application/json")
				srv.ServeHTTP(rec, req)
				var result struct {
					Compiled bool `json:"compiled"`
					Success  bool `json:"success"`
				}
				if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || rec.Code != http.StatusOK || !result.Compiled || !result.Success {
					t.Fatalf("tooling: %d %s, error %v", rec.Code, rec.Body.String(), err)
				}
			})
		})
	}
}
