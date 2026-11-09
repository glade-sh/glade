package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

// Native named cold and repeated USER_MODE observations: road174_overload_controls/org.tsv:3,9,12,18; 68 = 67.
func TestRoad174LightningRecordAccessRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeLightningFixtureFile(t, filepath.Join(root, "sfdx-project.json"), fmt.Sprintf(`{"sourceApiVersion":%q,"packageDirectories":[{"path":"force-app","default":true}]}`, api))
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/classes/AccessController.cls"), `public class AccessController { public String getValue(){return observe();} @AuraEnabled public static String observe() { Id uid=UserInfo.getUserId(); String q='SELECT UserRecordAccess.HasReadAccess,UserRecordAccess.HasEditAccess,UserRecordAccess.HasDeleteAccess,UserRecordAccess.HasTransferAccess,UserRecordAccess.HasAllAccess,UserRecordAccess.MaxAccessLevel FROM Order WHERE OwnerId = :uid'; List<SObject> a=Database.query(q,AccessLevel.USER_MODE); List<SObject> b=Database.query(q,AccessLevel.USER_MODE); SObject x=a[0].getSObject('UserRecordAccess'); SObject y=b[0].getSObject('UserRecordAccess'); return String.valueOf(x.get('HasAllAccess'))+'|'+String.valueOf(y.get('HasAllAccess'))+'|'+String.valueOf(x.get('MaxAccessLevel')); } }`)
			writeLightningFixtureFile(t, filepath.Join(root, "force-app/classes/AccessController.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			pageAPI := api
			if api == "62.0" {
				pageAPI = "59.0"
			}
			page := filepath.Join(root, "force-app/pages/AccessPage.page")
			writeLightningFixtureFile(t, page, `<apex:page controller="AccessController"><apex:outputText value="{!value}"/></apex:page>`)
			writeLightningFixtureFile(t, page+"-meta.xml", `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+pageAPI+`</apiVersion><label>Access</label></ApexPage>`)
			p, err := project.Load(root)
			if err != nil {
				t.Fatal(err)
			}
			objects, err := schema.LoadProject(p)
			if err != nil {
				t.Fatal(err)
			}
			source, err := NewSourceMetadataFromProject(p)
			if err != nil {
				t.Fatal(err)
			}
			org := testOrg()
			storage.EnsureDeterministicPlatformData(&org)
			storage.EnsureStandardObject(&org, "Order")
			org.APIVersion = api
			var profile storage.ID
			for id, row := range org.Objects["Profile"].Records {
				if row.Fields["Name"].String == "Standard User" {
					profile = id
					break
				}
			}
			if profile == "" {
				t.Fatal("profile missing")
			}
			const actor storage.ID = "005000000000099"
			const perm storage.ID = "0PS000000000099"
			const order storage.ID = "801000000000001"
			fs := storage.NewFixture()
			fs.Objects = []storage.FixtureObject{
				{Name: "User", Records: []storage.FixtureRecord{{ID: actor, Fields: map[string]storage.Value{"Username": storage.StringValue("proof@example.invalid"), "ProfileId": storage.IDValue(profile), "LastName": storage.StringValue("Proof")}}}},
				{Name: "PermissionSet", Records: []storage.FixtureRecord{{ID: perm, Fields: map[string]storage.Value{"Name": storage.StringValue("Proof")}}}},
				{Name: "ObjectPermissions", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"ParentId": storage.IDValue(perm), "SObjectType": storage.StringValue("Order"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true), "PermissionsViewAllRecords": storage.BooleanValue(true), "PermissionsModifyAllRecords": storage.BooleanValue(true)}}}},
				{Name: "PermissionSetAssignment", Records: []storage.FixtureRecord{{Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(actor), "PermissionSetId": storage.IDValue(perm)}}}},
			}
			if err := storage.ApplyFixture(&org, fs); err != nil {
				t.Fatal(err)
			}
			obj := org.Objects["Order"]
			obj.Definition.SharingModel = "ReadWrite"
			obj.Records = map[storage.ID]storage.Record{order: {ID: order, Object: "Order", Fields: map[string]storage.Value{"Description": storage.StringValue("proof")}, System: storage.SystemFields{OwnerID: actor}}}
			org.Objects["Order"] = obj
			handler := newLightningTestServer(t, &org, source)
			handler.SetProjectIndex(typesys.Build(p, objects))
			for _, route := range []struct{ path, body string }{{"/lightning/wire/apex", `{"className":"AccessController","method":"observe","params":{}}`}, {"/lightning/apex/AccessController/observe", `{}`}} {
				for pass := 0; pass < 2; pass++ {
					rec := httptest.NewRecorder()
					req := httptest.NewRequest(http.MethodPost, route.path, strings.NewReader(route.body))
					req.Header.Set("Content-Type", "application/json")
					req.Header.Set("X-GLADE-User-Id", string(actor))
					handler.ServeHTTP(rec, req)
					var out lwcbrowser.WireResponse
					if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
						t.Fatal(err)
					}
					if rec.Code != http.StatusOK || out.Error != nil || out.Data != "false|true|All" {
						t.Fatalf("%s pass %d status %d result %+v", route.path, pass, rec.Code, out)
					}
				}
			}
			vf := newVisualforceHTMLTestServer(t, &org, source)
			vf.VisualforceHTMLUserID = actor
			vf.SetProjectIndex(typesys.Build(p, objects))
			for pass := 0; pass < 2; pass++ {
				rec := httptest.NewRecorder()
				vf.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/AccessPage", nil))
				if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "false|true|All") {
					t.Fatalf("Visualforce pass %d status %d body %s", pass, rec.Code, rec.Body.String())
				}
			}
		})
	}
}
