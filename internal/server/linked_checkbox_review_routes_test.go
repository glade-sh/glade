package server

import (
	"encoding/csv"
	"encoding/json"
	"io"
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

// Replay the native caller value through local VF and LWC HTTP hosts. The
// Salesforce observations are from Apex, not a native browser-host capture.
func TestLinkedCheckboxReviewServerHostRoutes(t *testing.T) {
	fixture := filepath.Join("..", "vm", "testdata", "linked_checkbox_review")
	f, err := os.Open(filepath.Join(fixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	reader := csv.NewReader(f)
	reader.Comma = '\t'
	reader.FieldsPerRecord = -1
	want := map[string]string{}
	for {
		row, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatal(err)
		}
		if len(row) >= 5 && row[1] == "anonymous" && row[2] == "custom-null-true" {
			want[row[0]] = row[3] + ":" + row[4]
		}
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			expected, ok := want[api]
			if !ok {
				t.Fatalf("missing native row for %s", api)
			}
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			for _, name := range []string{"Parent__c/Parent__c.object-meta.xml", "Parent__c/fields/Flag__c.field-meta.xml", "Child__c/Child__c.object-meta.xml", "Child__c/fields/Parent__c.field-meta.xml"} {
				raw, err := os.ReadFile(filepath.Join(fixture, "objects", name))
				if err != nil {
					t.Fatal(err)
				}
				writeServerTestFile(t, filepath.Join(root, "force-app/main/default/objects", name), string(raw))
			}
			source := `public class CheckboxHost { @AuraEnabled public static String run() { Parent__c p=new Parent__c(Flag__c=null); Child__c c=new Child__c(Parent__r=p); return String.valueOf(c.Parent__r.Flag__c)+':'+String.valueOf(c.Parent__r.get('Flag__c')); } public String getOutcome(){return run();} }`
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/CheckboxHost.cls"), source)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/CheckboxHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/CheckboxHostPage.page"), `<apex:page controller="CheckboxHost" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/CheckboxHostPage.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><label>Checkbox host</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			metadata, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			org.Objects["Parent__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Parent__c", Fields: map[string]storage.Field{"Flag__c": {APIName: "Flag__c", Type: storage.FieldBoolean, DefaultValue: "true", Nillable: storage.BoolFlag(false)}}}, Records: map[storage.ID]storage.Record{}}
			org.Objects["Child__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Child__c", Fields: map[string]storage.Field{"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Parent__c"}, RelationshipName: "Parent__r"}}}, Records: map[storage.ID]storage.Record{}}
			handler := newLightningTestServer(t, &org, metadata)
			configureVisualforceTestPrincipal(t, handler)
			handler.SetProjectIndex(typesys.Build(p, schema))
			if handler.runtimeErr != nil {
				t.Fatal(handler.runtimeErr)
			}
			for _, route := range []struct{ name, method, path, body string }{
				{"vf", http.MethodGet, "/apex/CheckboxHostPage", ""},
				{"lwc-wire", http.MethodPost, "/lightning/wire/apex", `{"className":"CheckboxHost","method":"run","params":{}}`},
				{"lwc-imperative", http.MethodPost, "/lightning/apex/CheckboxHost/run", `{}`},
			} {
				t.Run(route.name, func(t *testing.T) {
					for _, state := range []string{"cold", "warm"} {
						t.Run(state, func(t *testing.T) {
							rec := httptest.NewRecorder()
							req := httptest.NewRequest(route.method, route.path, strings.NewReader(route.body))
							req.Header.Set("Content-Type", "application/json")
							handler.ServeHTTP(rec, req)
							if rec.Code != http.StatusOK {
								t.Fatalf("HTTP %d: %s", rec.Code, rec.Body.String())
							}
							if route.method == http.MethodGet {
								if !strings.Contains(rec.Body.String(), ">"+expected+"<") {
									t.Fatalf("VF response %s; native %s", rec.Body.String(), expected)
								}
								return
							}
							var out lwcbrowser.WireResponse
							if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
								t.Fatal(err)
							}
							if out.Error != nil || out.Data != expected {
								t.Fatalf("response=%+v; native=%s", out, expected)
							}
						})
					}
				})
			}
		})
	}
}
