package storage

import "testing"

// Existing fixture data must survive repeated default hydration and ID collisions.
func TestDefaultRoleSeedPreservesImportedAssignments(t *testing.T) {
	org := NewOrgState()
	EnsureStandardObject(&org, "UserRole")
	EnsureStandardObject(&org, "User")
	roleID := ID("00E000000000002")
	roleState := org.Objects["UserRole"]
	roleState.Records[roleID] = Record{ID: roleID, Object: "UserRole", Fields: map[string]Value{"DeveloperName": StringValue("OwnedRole"), "Name": StringValue("Owned Role")}}
	org.Objects["UserRole"] = roleState
	users := org.Objects["User"]
	users.Records[ID(DefaultUserID)] = Record{ID: ID(DefaultUserID), Object: "User", Fields: map[string]Value{"UserRoleId": IDValue(roleID)}}
	org.Objects["User"] = users
	for range 2 {
		EnsureDeterministicPlatformData(&org)
		role := org.Objects["UserRole"].Records[roleID]
		if role.Fields["DeveloperName"].String != "OwnedRole" || role.Fields["Name"].String != "Owned Role" {
			t.Fatalf("imported role changed: %+v", role)
		}
		if got := org.Objects["User"].Records[ID(DefaultUserID)].Fields["UserRoleId"].ID; got != roleID {
			t.Fatalf("imported assignment = %s", got)
		}
	}
}
