package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestStandardProfileAccountFallbackIdentityAndGrants(t *testing.T) {
	for _, name := range []string{"Work.com Only User", "Minimum Access - Salesforce"} {
		t.Run(name, func(t *testing.T) {
			org := testDataOrg()
			storage.EnsureDeterministicPlatformData(&org)
			var profileID storage.ID
			for id, row := range org.Objects["Profile"].Records {
				if row.Fields["Name"].String == name {
					profileID = id
					break
				}
			}
			if profileID == "" {
				t.Fatal("missing seeded profile")
			}
			machine := New(nil)
			machine.SetOrg(&org)
			actor := Object("User")
			actor.Fields["Id"] = String("005000000000091")
			actor.Fields["ProfileId"] = String(string(profileID))
			machine.executionUser = actor
			profile := org.Objects["Profile"].Records[profileID]
			profile.Fields["Name"] = storage.StringValue("Renamed seeded profile")
			org.Objects["Profile"].Records[profileID] = profile
			if machine.currentUserObjectPermission("Account", "isAccessible") {
				t.Error("ungranted seeded profile must deny Account after rename")
			}
			const setID storage.ID = "0PS000000000091"
			org.Objects["PermissionSet"].Records[setID] = storage.Record{ID: setID, Object: "PermissionSet", Fields: map[string]storage.Value{"IsCustom": storage.BooleanValue(true)}}
			org.Objects["PermissionSetAssignment"].Records["0Pa000000000091"] = storage.Record{ID: "0Pa000000000091", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue("005000000000091"), "PermissionSetId": storage.IDValue(setID)}}
			org.Objects["ObjectPermissions"].Records["110000000000091"] = storage.Record{ID: "110000000000091", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(setID), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true)}}
			if !machine.currentUserObjectPermission("Account", "isAccessible") {
				t.Error("assigned grant must allow Account")
			}
		})
	}
	t.Run("custom colliding name", func(t *testing.T) {
		org := testDataOrg()
		storage.EnsureDeterministicPlatformData(&org)
		const profileID storage.ID = "00e000000000091"
		const setID storage.ID = "0PS000000000091"
		org.Objects["Profile"].Records[profileID] = storage.Record{ID: profileID, Object: "Profile", Fields: map[string]storage.Value{"Name": storage.StringValue("Work.com Only User")}}
		org.Objects["PermissionSet"].Records[setID] = storage.Record{ID: setID, Object: "PermissionSet", Fields: map[string]storage.Value{"ProfileId": storage.IDValue(profileID), "IsOwnedByProfile": storage.BooleanValue(true), "IsCustom": storage.BooleanValue(true)}}
		machine := New(nil)
		machine.SetOrg(&org)
		actor := Object("User")
		actor.Fields["ProfileId"] = String(string(profileID))
		machine.executionUser = actor
		if !machine.currentUserObjectPermission("Account", "isAccessible") {
			t.Error("custom profile must retain fallback despite colliding name")
		}
		org.Objects["ObjectPermissions"].Records["110000000000091"] = storage.Record{ID: "110000000000091", Object: "ObjectPermissions", Fields: map[string]storage.Value{"ParentId": storage.IDValue(setID), "SObjectType": storage.StringValue("Account"), "PermissionsRead": storage.BooleanValue(true)}}
		if !machine.currentUserObjectPermission("Account", "isAccessible") {
			t.Error("custom profile grant must allow Account")
		}
	})
}
