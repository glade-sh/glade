package server

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/lwcbrowser"
	"github.com/glade-sh/glade/internal/project"
	gladeschema "github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestApprovalLockedGrantHostRoutes(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			writeServerTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"`+api+`"}`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/LockedHost.cls"), `public class LockedHost {
   @AuraEnabled public static String run() { Account a=new Account(Name='Probe');insert a;Approval.lock(a);a.Name='Changed';Database.SaveResult r=Database.update(a,false);return String.valueOf(r.isSuccess())+':'+(r.isSuccess()?'NONE':String.valueOf(r.getErrors()[0].getStatusCode())); }
   public String getOutcome() {return run();}
  }`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/classes/LockedHost.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>`+api+`</apiVersion><status>Active</status></ApexClass>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/LockedHost.page"), `<apex:page controller="LockedHost" showHeader="false" sidebar="false"><apex:outputText value="{!outcome}"/></apex:page>`)
			writeServerTestFile(t, filepath.Join(root, "force-app/main/default/pages/LockedHost.page-meta.xml"), `<ApexPage xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>59.0</apiVersion><label>Locked host</label></ApexPage>`)
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
			index := typesys.Build(p, schema)
			org := apextest.OrgFromIndex(index)
			var profile storage.ID
			for id, row := range org.Objects["Profile"].Records {
				if row.Fields["Name"].String == "Standard User" {
					profile = id
					break
				}
			}
			if profile == "" {
				t.Fatal("standard profile missing")
			}
			uid := storage.ID("005000000000991AAA")
			users := org.Objects["User"]
			users.Records[uid] = storage.Record{ID: uid, Object: "User", Fields: map[string]storage.Value{"Id": storage.IDValue(uid), "ProfileId": storage.IDValue(profile)}}
			org.Objects["User"] = users
			parent := profile
			for id, set := range org.Objects["PermissionSet"].Records {
				if set.Fields["IsOwnedByProfile"].Boolean && storage.IDsEqual(set.Fields["ProfileId"].ID, profile) {
					parent = id
					break
				}
			}
			storage.EnsureStandardObject(&org, "ObjectPermissions")
			perms := org.Objects["ObjectPermissions"]
			for id, row := range perms.Records {
				objectType, _ := row.GetField("SObjectType")
				if storage.IDsEqual(row.Fields["ParentId"].ID, parent) && strings.EqualFold(objectType.String, "Account") {
					delete(perms.Records, id)
				}
			}
			permID := storage.ID("110000000000991AAA")
			perms.Records[permID] = storage.Record{ID: permID, Object: "ObjectPermissions", Fields: map[string]storage.Value{"Id": storage.IDValue(permID), "ParentId": storage.IDValue(parent), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true), "PermissionsCreate": storage.BooleanValue(true), "PermissionsEdit": storage.BooleanValue(true), "PermissionsDelete": storage.BooleanValue(true)}}
			org.Objects["ObjectPermissions"] = perms
			for _, state := range []string{"object_muted", "both_muted", "direct_global", "revoked_global", "expired"} {
				t.Run(state, func(t *testing.T) {
					stateOrg := org.Clone()
					approvalGrantHostSetup(&stateOrg, uid, state)
					want := "true:NONE"
					if state == "both_muted" || state == "revoked_global" || state == "expired" {
						want = "false:ENTITY_IS_LOCKED"
					}
					handler := newLightningTestServer(t, &stateOrg, source)
					handler.VisualforceHTMLUserID = uid
					handler.SetProjectIndex(index)
					if handler.runtimeErr != nil {
						t.Fatal(handler.runtimeErr)
					}
					for _, phase := range []string{"cold", "warm"} {
						t.Run(phase, func(t *testing.T) {
							rec := httptest.NewRecorder()
							handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/apex/LockedHost", nil))
							if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), ">"+want+"<") {
								t.Fatalf("VF status=%d body=%s", rec.Code, rec.Body.String())
							}
							rec = httptest.NewRecorder()
							req := httptest.NewRequest(http.MethodPost, "/lightning/apex/LockedHost/run", strings.NewReader(`{}`))
							req.Header.Set("Content-Type", "application/json")
							req.Header.Set("X-GLADE-User-Id", string(uid))
							handler.ServeHTTP(rec, req)
							var out lwcbrowser.WireResponse
							if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
								t.Fatal(err)
							}
							if rec.Code != http.StatusOK || out.Error != nil || out.Data != want {
								t.Fatalf("LWC status=%d response=%+v", rec.Code, out)
							}
						})
					}
				})
			}
		})
	}
}

// The permission state mirrors captured anonymous Apex rows; both host routes
// execute the same locked update in cold and warm server state.
func approvalGrantHostSetup(org *storage.OrgState, uid storage.ID, state string) {
	for _, name := range []string{"PermissionSetAssignment", "PermissionSetGroupComponent", "MutingPermissionSet", "ObjectPermissions"} {
		storage.EnsureStandardObject(org, name)
	}
	group := storage.ID("0PG000000000991AAA")
	set := storage.ID("0PS000000000991AAA")
	mute := storage.ID("0PS000000000992AAA")
	direct := storage.ID("0PS000000000993AAA")
	sets := org.Objects["PermissionSet"]
	sets.Records[set] = storage.Record{ID: set, Object: "PermissionSet", Fields: map[string]storage.Value{"PermissionsModifyAllData": storage.BooleanValue(true)}}
	sets.Records[direct] = storage.Record{ID: direct, Object: "PermissionSet", Fields: map[string]storage.Value{"PermissionsModifyAllData": storage.BooleanValue(true)}}
	org.Objects["PermissionSet"] = sets
	mutings := org.Objects["MutingPermissionSet"]
	mutings.Records[mute] = storage.Record{ID: mute, Object: "MutingPermissionSet", Fields: map[string]storage.Value{"PermissionsModifyAllData": storage.BooleanValue(state != "object_muted")}}
	org.Objects["MutingPermissionSet"] = mutings
	perms := org.Objects["ObjectPermissions"]
	for i, parent := range []storage.ID{set, mute, direct} {
		id := storage.ID([]string{"110000000000992AAA", "110000000000993AAA", "110000000000994AAA"}[i])
		perms.Records[id] = storage.Record{ID: id, Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(parent), "SObjectType": storage.StringValue("Account"), "PermissionsModifyAllRecords": storage.BooleanValue(true)}}
	}
	org.Objects["ObjectPermissions"] = perms
	components := org.Objects["PermissionSetGroupComponent"]
	for i, parent := range []storage.ID{set, mute} {
		id := storage.ID([]string{"0PC000000000991AAA", "0PC000000000992AAA"}[i])
		components.Records[id] = storage.Record{ID: id, Object: "PermissionSetGroupComponent", Fields: map[string]storage.Value{"PermissionSetGroupId": storage.IDValue(group), "PermissionSetId": storage.IDValue(parent)}}
	}
	org.Objects["PermissionSetGroupComponent"] = components
	assignments := org.Objects["PermissionSetAssignment"]
	a := storage.ID("0Pa000000000991AAA")
	assignments.Records[a] = storage.Record{ID: a, Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(uid), "PermissionSetGroupId": storage.IDValue(group)}}
	if state == "direct_global" || state == "revoked_global" || state == "expired" {
		id := storage.ID("0Pa000000000992AAA")
		fields := map[string]storage.Value{"AssigneeId": storage.IDValue(uid), "PermissionSetId": storage.IDValue(direct), "IsRevoked": storage.BooleanValue(state == "revoked_global")}
		if state == "expired" {
			fields["ExpirationDate"] = storage.DateTimeValue("2025-01-01T00:00:00Z")
		}
		assignments.Records[id] = storage.Record{ID: id, Object: "PermissionSetAssignment", Fields: fields}
	}
	org.Objects["PermissionSetAssignment"] = assignments
}
