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

// The host transports the anonymous and named observations from
// pr163_fieldset_keys/org.tsv; these are supplemental local route checks.
func TestFieldSetKeyVisualforceLightningAndToolingHosts(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"namespace":"gladetest1","packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="FieldSetHost"></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>59.0</apiVersion><label>Field Set Host</label></ApexPage>`)
			const probe = `Map<String, Schema.FieldSet> sets = Account.SObjectType.getDescribe().fieldSets.getMap();
System.assertEquals(1, sets.size());
System.assert(sets.keySet().contains('gladetest1__codexc163capture'));
System.assert(sets.containsKey('GLADETEST1__CODEXC163CAPTURE'));
System.assertNotEquals(null, sets.get('gladetest1__CodexC163Capture'));
System.assertEquals(false, sets.containsKey('CodexC163Capture'));
System.assertEquals(null, sets.get('CodexC163Capture'));`
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/FieldSetHost.cls"), `public class FieldSetHost { @AuraEnabled @RemoteAction public static String observe() { `+probe+` return 'ok'; } }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/FieldSetHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
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
			org.Namespace = "gladetest1"
			storage.EnsureStandardObject(&org, "Account")
			org.Metadata.FieldSets = []storage.FieldSetMetadata{{ObjectName: "Account", Name: "CodexC163Capture"}}
			srv := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, srv)
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			t.Run("visualforce", func(t *testing.T) {
				for pass := 0; pass < 2; pass++ {
					responses := postVisualforceRemotingWithViewState(t, srv, `[{"action":"FieldSetHost","method":"observe","data":[],"type":"rpc","tid":3}]`)
					if len(responses) != 1 || !responses[0].Status || responses[0].Result != "ok" {
						t.Fatalf("pass %d: remoting: %+v", pass, responses)
					}
				}
			})
			t.Run("lightning", func(t *testing.T) {
				for pass := 0; pass < 2; pass++ {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", strings.NewReader(`{"className":"FieldSetHost","method":"observe","params":{}}`))
					req.Header.Set("Content-Type", "application/json")
					srv.ServeHTTP(rec, req)
					var body struct {
						Data  any `json:"data"`
						Error any `json:"error"`
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || rec.Code != http.StatusOK || body.Error != nil || body.Data != "ok" {
						t.Fatalf("pass %d: wire: %d %s, error %v", pass, rec.Code, rec.Body.String(), err)
					}
				}
			})
			t.Run("tooling", func(t *testing.T) {
				body, err := json.Marshal(map[string]string{"anonymousBody": probe})
				if err != nil {
					t.Fatal(err)
				}
				for pass := 0; pass < 2; pass++ {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", bytes.NewReader(body))
					req.Header.Set("Content-Type", "application/json")
					srv.ServeHTTP(rec, req)
					var result struct {
						Compiled bool `json:"compiled"`
						Success  bool `json:"success"`
					}
					if err := json.Unmarshal(rec.Body.Bytes(), &result); err != nil || rec.Code != http.StatusOK || !result.Compiled || !result.Success {
						t.Fatalf("pass %d: tooling: %d %s, error %v", pass, rec.Code, rec.Body.String(), err)
					}
				}
			})
		})
	}
}

// These are local transport checks of the anonymous and IsTest native rows;
// they do not claim that any host route was captured in Salesforce.
func TestSchemaDescribeVisualforceLightningAndToolingHosts(t *testing.T) {
	cases := []struct{ method, source string }{
		{"location", `Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
System.assertEquals(true, fields.containsKey('Probe_Point__c'));
System.assertEquals(true, fields.containsKey('Probe_Point__Latitude__s'));
System.assertEquals(true, fields.containsKey('probe_point__longitude__s'));
System.assertEquals('Probe_Point__Latitude__s', fields.get('Probe_Point__Latitude__s').getDescribe().getName());
System.assertEquals('Probe_Point__c', fields.get('Probe_Point__Latitude__s').getDescribe().getCompoundFieldName());
System.assertEquals('Probe_Point__c', fields.get('probe_point__longitude__s').getDescribe().getCompoundFieldName());
System.assertEquals(false, fields.containsKey('Probe_Text__Latitude__s'));
System.assertEquals(false, fields.containsKey('Absent__Latitude__s'));`},
		{"defaults", `Map<String, Schema.SObjectField> fields = Account.SObjectType.getDescribe().fields.getMap();
System.assertEquals(null, fields.get('Blank__c').getDescribe().getDefaultValue());
System.assertEquals(null, fields.get('Number__c').getDescribe().getDefaultValue());
System.assertEquals(true, fields.get('Flag__c').getDescribe().getDefaultValue());
System.assertEquals(false, fields.get('FlagFalse__c').getDescribe().getDefaultValue());
System.assertEquals(null, Integer.valueOf(fields.get('Blank__c').getDescribe().getDefaultValue()));`},
		{"shorthand", `System.assertEquals('Name', Account.SObjectType.Name.getDescribe().getName());
System.assertEquals('Id', Account.SObjectType.Id.getDescribe().getName());
System.assertEquals(1, SObjectType.Probe__c.FieldSets.Main.getFields().size());
System.assertEquals(1, Schema.SObjectType.Probe__c.FieldSets.MainList.getFields().size());`},
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":%q}`, api))
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page"), `<apex:page controller="SchemaHost"></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/Remote.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>59.0</apiVersion><label>Schema Host</label></ApexPage>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/objects/Probe__c/Probe__c.object-meta.xml"), `<CustomObject xmlns="http://soap.sforce.com/2006/04/metadata"><label>Probe</label><pluralLabel>Probes</pluralLabel><nameField><type>Text</type><label>Probe Name</label></nameField><deploymentStatus>Deployed</deploymentStatus><sharingModel>ReadWrite</sharingModel></CustomObject>`)
			for _, name := range []string{"Main", "MainList"} {
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/objects/Probe__c/fieldSets", name+".fieldSet-meta.xml"), `<FieldSet xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>`+name+`</fullName><label>`+name+`</label><description>Probe</description><displayedFields><field>Name</field><isFieldManaged>false</isFieldManaged><isRequired>false</isRequired></displayedFields></FieldSet>`)
			}
			classSource := `public class SchemaHost {`
			for _, tc := range cases {
				classSource += ` @AuraEnabled @RemoteAction public static String ` + tc.method + `() { ` + tc.source + ` return 'ok'; }`
			}
			classSource += ` }`
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/SchemaHost.cls"), classSource)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/SchemaHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
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
			storage.EnsureStandardObject(&org, "Account")
			account := org.Objects["Account"]
			for name, field := range map[string]storage.Field{
				"Probe_Point__c": {APIName: "Probe_Point__c", Type: storage.FieldLocation, DisplayType: "LOCATION"},
				"Probe_Text__c":  {APIName: "Probe_Text__c", Type: storage.FieldString},
				"Blank__c":       {APIName: "Blank__c", Type: storage.FieldString},
				"Number__c":      {APIName: "Number__c", Type: storage.FieldDecimal, DisplayType: "DOUBLE", DefaultValue: "7", DefaultValueFromMetadata: true},
				"Flag__c":        {APIName: "Flag__c", Type: storage.FieldBoolean, DefaultValue: "true", DefaultValueFromMetadata: true},
				"FlagFalse__c":   {APIName: "FlagFalse__c", Type: storage.FieldBoolean, DefaultValue: "false", DefaultValueFromMetadata: true},
			} {
				account.Definition.Fields[name] = field
			}
			org.Objects["Account"] = account
			org.Objects["Probe__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{APIName: "Probe__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}},
				Records:    map[storage.ID]storage.Record{},
			}
			org.Metadata.FieldSets = []storage.FieldSetMetadata{
				{ObjectName: "Probe__c", Name: "Main", Fields: []storage.FieldSetMemberMetadata{{Field: "Name"}}},
				{ObjectName: "Probe__c", Name: "MainList", Fields: []storage.FieldSetMemberMetadata{{Field: "Name"}}},
			}
			srv := newLightningTestServer(t, &org, source)
			configureVisualforceTestPrincipal(t, srv)
			srv.SetProjectIndex(typesys.Build(p, schema))
			if srv.runtimeErr != nil {
				t.Fatal(srv.runtimeErr)
			}
			for _, tc := range cases {
				t.Run(tc.method, func(t *testing.T) {
					t.Run("visualforce", func(t *testing.T) {
						body, err := json.Marshal([]map[string]any{{"action": "SchemaHost", "method": tc.method, "data": []any{}, "type": "rpc", "tid": 3}})
						if err != nil {
							t.Fatal(err)
						}
						responses := postVisualforceRemotingWithViewState(t, srv, string(body))
						if len(responses) != 1 || !responses[0].Status || responses[0].Result != "ok" {
							t.Fatalf("remoting: %+v", responses)
						}
					})
					t.Run("lightning", func(t *testing.T) {
						body, err := json.Marshal(map[string]any{"className": "SchemaHost", "method": tc.method, "params": map[string]any{}})
						if err != nil {
							t.Fatal(err)
						}
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(http.MethodPost, "/lightning/wire/apex", bytes.NewReader(body))
						req.Header.Set("Content-Type", "application/json")
						srv.ServeHTTP(rec, req)
						var got struct {
							Data  any `json:"data"`
							Error any `json:"error"`
						}
						if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK || got.Error != nil || got.Data != "ok" {
							t.Fatalf("wire: %d %s, error %v", rec.Code, rec.Body.String(), err)
						}
					})
					t.Run("tooling", func(t *testing.T) {
						body, err := json.Marshal(map[string]string{"anonymousBody": tc.source})
						if err != nil {
							t.Fatal(err)
						}
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(http.MethodPost, "/services/data/v"+api+"/tooling/executeAnonymous", bytes.NewReader(body))
						req.Header.Set("Content-Type", "application/json")
						srv.ServeHTTP(rec, req)
						var got struct {
							Compiled bool `json:"compiled"`
							Success  bool `json:"success"`
						}
						if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil || rec.Code != http.StatusOK || !got.Compiled || !got.Success {
							t.Fatalf("tooling: %d %s, error %v", rec.Code, rec.Body.String(), err)
						}
					})
				})
			}
		})
	}
}
