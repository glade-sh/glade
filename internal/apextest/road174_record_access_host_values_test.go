package apextest_test

import (
	"encoding/json"
	"fmt"
	"html"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Replay populated anonymous native values through local controller adapters.
func TestRoad174RecordAccessHostValues(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			write := func(name, body string) {
				t.Helper()
				p := filepath.Join(root, name)
				if err := os.MkdirAll(filepath.Dir(p), 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(p, []byte(body), 0644); err != nil {
					t.Fatal(err)
				}
			}
			write("sfdx-project.json", fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			write("force-app/main/default/classes/AccessValueHost.cls", `public class AccessValueHost {
 @AuraEnabled(cacheable=true) public static String observe(String objectName) {
  if(objectName=='NestedOrder') {
   SObject row=Database.query('SELECT Parent__r.UserRecordAccess.HasAllAccess,Parent__r.UserRecordAccess.MaxAccessLevel FROM Road174Child__c WITH SYSTEM_MODE')[0];
   SObject a=row.getSObject('Parent__r').getSObject('UserRecordAccess');
   return JSON.serialize(new List<Object>{a.get('HasAllAccess'),a.get('MaxAccessLevel')});
  }
  if(objectName=='OrderScoped') {
   try {
    AccessLevel level=AccessLevel.USER_MODE.withPermissionSetId('0PS000000000099');
    Database.query('SELECT UserRecordAccess.HasAllAccess FROM Order',level);
    return JSON.serialize(new List<Object>{'unexpected-success'});
   } catch(Exception e) { return JSON.serialize(new List<Object>{e.getTypeName()}); }
  }
  if(objectName=='OrderArgument'||objectName=='OrderGroup') {
   Id uid=UserInfo.getUserId();
   String q='SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Order WHERE OwnerId = :uid';
   SObject first=Database.query(q,AccessLevel.USER_MODE)[0].getSObject('UserRecordAccess');
   Object firstValue=first.get('HasAllAccess');
   SObject second=Database.query(q,AccessLevel.USER_MODE)[0].getSObject('UserRecordAccess');
   return JSON.serialize(new List<Object>{firstValue,second.get('HasAllAccess'),second.get('MaxAccessLevel')});
  }
  List<SObject> records=Database.query('SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM '+objectName+(objectName=='Order'?' WITH SYSTEM_MODE':'')+' LIMIT 1');
  SObject a=records[0].getSObject('UserRecordAccess');
  return JSON.serialize(new List<Object>{a.get('HasReadAccess'),a.get('HasEditAccess'),a.get('HasDeleteAccess'),a.get('HasTransferAccess'),a.get('HasAllAccess'),a.get('MaxAccessLevel')});
 }
 public String getObservation() { return observe(ApexPages.currentPage().getParameters().get('objectName')); }
}`)
			write("force-app/main/default/classes/AccessValueHost.cls-meta.xml", `<ApexClass><apiVersion>`+api+`</apiVersion></ApexClass>`)
			write("force-app/main/default/pages/AccessValueHost.page", `<apex:page controller="AccessValueHost" showHeader="false" sidebar="false"><pre id="observation"><apex:outputText value="{!observation}"/></pre></apex:page>`)
			write("force-app/main/default/pages/AccessValueHost.page-meta.xml", `<ApexPage><apiVersion>`+api+`</apiVersion><label>Access values</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			schema, err := gladeschema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			source, err := server.NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			org.APIVersion = api
			org.Features = append(org.Features, "ApexUserModeWithPermset")
			storage.EnsureDeterministicPlatformData(&org)
			for _, name := range []string{"Asset", "Pricebook2", "Order"} {
				storage.EnsureStandardObject(&org, name)
				obj := org.Objects[name]
				id := storage.ID(obj.Definition.KeyPrefix + "000000000001")
				obj.Records = map[storage.ID]storage.Record{id: {ID: id, Object: name, Fields: map[string]storage.Value{}, System: storage.SystemFields{OwnerID: storage.DefaultUserID}}}
				org.Objects[name] = obj
			}
			orderID := storage.ID(org.Objects["Order"].Definition.KeyPrefix + "000000000001")
			org.Objects["Road174Child__c"] = storage.ObjectState{
				Definition: storage.ObjectDefinition{APIName: "Road174Child__c", Fields: map[string]storage.Field{
					"Id":        {APIName: "Id", Type: storage.FieldID},
					"Parent__c": {APIName: "Parent__c", Type: storage.FieldReference, ReferenceTo: []string{"Order"}},
				}, Relations: []storage.Relationship{{Field: "Parent__c", ParentObjects: []string{"Order"}, ParentRelationship: "Parent__r"}}},
				Records: map[storage.ID]storage.Record{"a01000000000001": {ID: "a01000000000001", Object: "Road174Child__c", Fields: map[string]storage.Value{"Parent__c": storage.IDValue(orderID)}, System: storage.SystemFields{OwnerID: storage.DefaultUserID}}},
			}
			const grantActor storage.ID = "005000000000099"
			const grantSet storage.ID = "0PS000000000099"
			const globalActor storage.ID = "005000000000098"
			const globalSet storage.ID = "0PS000000000098"
			const groupActor storage.ID = "005000000000097"
			const groupID storage.ID = "0PG000000000099"
			var profile storage.ID
			for id, record := range org.Objects["Profile"].Records {
				if record.Fields["Name"].String == "Standard User" {
					profile = id
					break
				}
			}
			if profile == "" {
				t.Fatal("missing standard profile")
			}
			fixture := storage.NewFixture()
			fixture.Objects = []storage.FixtureObject{
				{Name: "User", Records: []storage.FixtureRecord{{ID: grantActor, Fields: map[string]storage.Value{"Username": storage.StringValue("proof@example.invalid"), "ProfileId": storage.IDValue(profile), "LastName": storage.StringValue("Proof")}}, {ID: globalActor, Fields: map[string]storage.Value{"Username": storage.StringValue("global@example.invalid"), "ProfileId": storage.IDValue(profile), "LastName": storage.StringValue("Proof")}}}},
				{Name: "PermissionSet", Records: []storage.FixtureRecord{{ID: grantSet, Fields: map[string]storage.Value{"Name": storage.StringValue("Proof"), "Label": storage.StringValue("Proof")}}, {ID: globalSet, Fields: map[string]storage.Value{"Name": storage.StringValue("GlobalProof"), "PermissionsModifyAllData": storage.BooleanValue(true)}}}},
				{Name: "ObjectPermissions", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"ParentId": storage.IDValue(grantSet), "SObjectType": storage.StringValue("Order"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true), "PermissionsViewAllRecords": storage.BooleanValue(true), "PermissionsModifyAllRecords": storage.BooleanValue(true)}}, {Fields: map[string]storage.Value{"ParentId": storage.IDValue(globalSet), "SObjectType": storage.StringValue("Order"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true), "PermissionsViewAllRecords": storage.BooleanValue(true), "PermissionsModifyAllRecords": storage.BooleanValue(true)}}}},
				{Name: "PermissionSetAssignment", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(grantActor), "PermissionSetId": storage.IDValue(grantSet)}}, {Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(globalActor), "PermissionSetId": storage.IDValue(globalSet)}}}},
			}
			if err := storage.ApplyFixture(&org, fixture); err != nil {
				t.Fatal(err)
			}
			// The group-only actor is a local preservation control, not native data.
			storage.EnsureStandardObject(&org, "PermissionSetGroupComponent")
			users := org.Objects["User"]
			users.Records[groupActor] = storage.Record{ID: groupActor, Object: "User", Fields: map[string]storage.Value{"Username": storage.StringValue("group@example.invalid"), "ProfileId": storage.IDValue(profile), "LastName": storage.StringValue("Proof")}}
			org.Objects["User"] = users
			assignments := org.Objects["PermissionSetAssignment"]
			assignID := storage.ID("0Pa000000000097")
			assignments.Records[assignID] = storage.Record{ID: assignID, Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(groupActor), "PermissionSetGroupId": storage.IDValue(groupID)}}
			org.Objects["PermissionSetAssignment"] = assignments
			components := org.Objects["PermissionSetGroupComponent"]
			componentID := storage.ID("0PC000000000097")
			components.Records[componentID] = storage.Record{ID: componentID, Object: "PermissionSetGroupComponent", Fields: map[string]storage.Value{"PermissionSetGroupId": storage.IDValue(groupID), "PermissionSetId": storage.IDValue(grantSet)}}
			org.Objects["PermissionSetGroupComponent"] = components
			handler := server.NewWithSource(&org, source)
			handler.VisualforceHTMLUserID = storage.ID(storage.DefaultUserID)
			handler.SetProjectIndex(typesys.Build(p, schema))
			cases := []struct {
				name  string
				want  []any
				actor storage.ID
			}{{"Asset", []any{true, true, true, true, false, "Transfer"}, ""}, {"Pricebook2", []any{true, true, true, false, false, "Delete"}, ""}}
			if api == "67.0" {
				cases = append(cases, struct {
					name  string
					want  []any
					actor storage.ID
				}{"Order", []any{true, true, true, true, false, "All"}, ""}, struct {
					name  string
					want  []any
					actor storage.ID
				}{"Order", []any{true, true, true, true, true, "All"}, grantActor}, struct {
					name  string
					want  []any
					actor storage.ID
				}{"Order", []any{true, true, true, true, true, "All"}, globalActor})
			}
			cases = append(cases, struct {
				name  string
				want  []any
				actor storage.ID
			}{"NestedOrder", []any{true, "All"}, ""})
			for _, actor := range []storage.ID{grantActor, globalActor} {
				cases = append(cases, struct {
					name  string
					want  []any
					actor storage.ID
				}{"OrderArgument", []any{false, true, "All"}, actor})
			}
			cases = append(cases, struct {
				name  string
				want  []any
				actor storage.ID
			}{"OrderScoped", []any{"System.QueryException"}, grantActor})
			cases = append(cases, struct {
				name  string
				want  []any
				actor storage.ID
			}{"OrderGroup", []any{true, true, "All"}, groupActor})
			for _, tc := range cases {
				for _, route := range []string{"vf", "imperative", "wire"} {
					caseName := tc.name
					if tc.actor == grantActor {
						caseName += "Grant"
					} else if tc.actor == globalActor {
						caseName += "Global"
					}
					t.Run(caseName+"/"+route, func(t *testing.T) {
						method, path, body := http.MethodGet, "/apex/AccessValueHost?"+url.Values{"objectName": {tc.name}}.Encode(), []byte(nil)
						if route != "vf" {
							method = http.MethodPost
							path = "/lightning/apex/AccessValueHost/observe"
							body, _ = json.Marshal(map[string]string{"objectName": tc.name})
							if route == "wire" {
								path = "/lightning/wire/apex"
								body, _ = json.Marshal(map[string]any{"className": "AccessValueHost", "method": "observe", "params": map[string]string{"objectName": tc.name}})
							}
						}
						rec := httptest.NewRecorder()
						req := httptest.NewRequest(method, path, strings.NewReader(string(body)))
						req.Header.Set("Content-Type", "application/json")
						if tc.name == "OrderArgument" || tc.name == "OrderGroup" {
							obj := org.Objects["Order"]
							for id, record := range obj.Records {
								record.System.OwnerID = tc.actor
								obj.Records[id] = record
							}
							org.Objects["Order"] = obj
						}
						handler.VisualforceHTMLUserID = storage.ID(storage.DefaultUserID)
						if tc.actor != "" {
							handler.VisualforceHTMLUserID = tc.actor
							req.Header.Set("X-GLADE-User-Id", string(tc.actor))
						}
						handler.ServeHTTP(rec, req)
						if rec.Code != http.StatusOK {
							t.Fatalf("status %d: %s", rec.Code, rec.Body.String())
						}
						var raw string
						if route == "vf" {
							_, after, ok := strings.Cut(rec.Body.String(), `<pre id="observation">`)
							v, _, closed := strings.Cut(after, "</pre>")
							if !ok || !closed {
								t.Fatalf("missing value: %s", rec.Body.String())
							}
							raw = html.UnescapeString(v)
						} else {
							var response lwcbrowser.WireResponse
							if err := json.Unmarshal(rec.Body.Bytes(), &response); err != nil {
								t.Fatal(err)
							}
							if response.Error != nil {
								t.Fatalf("bridge: %+v", response.Error)
							}
							raw, _ = response.Data.(string)
						}
						var got []any
						if err := json.Unmarshal([]byte(raw), &got); err != nil {
							t.Fatalf("value %q: %v", raw, err)
						}
						if !reflect.DeepEqual(got, tc.want) {
							t.Fatalf("got %v want %v", got, tc.want)
						}
					})
				}
			}
		})
	}
}
