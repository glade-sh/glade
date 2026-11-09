package dml

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestApprovalLockedMutationPermissions(t *testing.T) {
	for _, grant := range []string{"none", "profile", "permission_set", "object", "group", "deleted_set", "deleted_object", "deleted_group", "deleted_profile"} {
		for _, op := range []string{"update", "delete", "upsert"} {
			t.Run(grant+"/"+op, func(t *testing.T) {
				org := testOrg()
				for _, name := range []string{"User", "Profile", "PermissionSet", "PermissionSetAssignment", "PermissionSetGroupComponent", "ObjectPermissions"} {
					storage.EnsureStandardObject(&org, name)
				}
				uid := storage.ID("005000000000991AAA")
				pid := storage.ID("00e000000000991AAA")
				setID := storage.ID("0PS000000000991AAA")
				users := org.Objects["User"]
				users.Records[uid] = storage.Record{ID: uid, Object: "User", Fields: map[string]storage.Value{"ProfileId": storage.IDValue(pid)}}
				org.Objects["User"] = users
				profiles := org.Objects["Profile"]
				profiles.Records[pid] = storage.Record{ID: pid, Object: "Profile", Fields: map[string]storage.Value{"PermissionsModifyAllData": storage.BooleanValue(grant == "profile" || grant == "deleted_profile")}}
				if grant == "deleted_profile" {
					row := profiles.Records[pid]
					row.System.IsDeleted = true
					profiles.Records[pid] = row
				}
				org.Objects["Profile"] = profiles
				sets := org.Objects["PermissionSet"]
				sets.Records[setID] = storage.Record{ID: setID, Object: "PermissionSet", Fields: map[string]storage.Value{"PermissionsModifyAllData": storage.BooleanValue(grant == "permission_set" || grant == "group" || grant == "deleted_set" || grant == "deleted_group")}}
				if grant == "deleted_set" {
					row := sets.Records[setID]
					row.System.IsDeleted = true
					sets.Records[setID] = row
				}
				org.Objects["PermissionSet"] = sets
				assigns := org.Objects["PermissionSetAssignment"]
				assigns.Records["0Pa000000000991AAA"] = storage.Record{ID: "0Pa000000000991AAA", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(uid), "PermissionSetId": storage.IDValue(setID)}}
				if grant == "group" || grant == "deleted_group" {
					groupID := storage.ID("0PG000000000991AAA")
					assignment := assigns.Records["0Pa000000000991AAA"]
					delete(assignment.Fields, "PermissionSetId")
					assignment.Fields["PermissionSetGroupId"] = storage.IDValue(groupID)
					assigns.Records[assignment.ID] = assignment
					components := org.Objects["PermissionSetGroupComponent"]
					components.Records["0PC000000000991AAA"] = storage.Record{ID: "0PC000000000991AAA", Object: "PermissionSetGroupComponent", Fields: map[string]storage.Value{"PermissionSetGroupId": storage.IDValue(groupID), "PermissionSetId": storage.IDValue(setID)}}
					if grant == "deleted_group" {
						row := components.Records["0PC000000000991AAA"]
						row.System.IsDeleted = true
						components.Records[row.ID] = row
					}
					org.Objects["PermissionSetGroupComponent"] = components
				}
				org.Objects["PermissionSetAssignment"] = assigns
				if grant == "object" || grant == "deleted_object" {
					perms := org.Objects["ObjectPermissions"]
					perms.Records["110000000000991AAA"] = storage.Record{ID: "110000000000991AAA", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(setID), "SObjectType": storage.StringValue("Account"), "PermissionsModifyAllRecords": storage.BooleanValue(true)}}
					if grant == "deleted_object" {
						row := perms.Records["110000000000991AAA"]
						row.System.IsDeleted = true
						perms.Records[row.ID] = row
					}
					org.Objects["ObjectPermissions"] = perms
				}
				engine := NewEngine(&org)
				created := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Probe")}}})
				if !created[0].Success {
					t.Fatal(created)
				}
				id := created[0].ID
				if result := engine.Lock([]storage.Record{{Object: "Account", ID: id}}); !result[0].Success {
					t.Fatal(result)
				}
				engine.UserID = uid
				input := []storage.Record{{Object: "Account", ID: id, Fields: map[string]storage.Value{"Name": storage.StringValue("Changed")}}}
				var result []Result
				switch op {
				case "update":
					result = engine.Update(input)
				case "delete":
					result = engine.Delete(input)
				case "upsert":
					result = engine.Upsert(input)
				}
				if grant == "none" || strings.HasPrefix(grant, "deleted_") {
					if result[0].Success || result[0].StatusCode != "ENTITY_IS_LOCKED" {
						t.Fatalf("result=%+v", result[0])
					}
				} else if !result[0].Success {
					t.Fatalf("result=%+v", result[0])
				}
			})
		}
	}
}
