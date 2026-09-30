package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestCustomPermissionAssignmentLifecycle(t *testing.T) {
	org := storage.OrgState{Objects: map[string]storage.ObjectState{}}
	put := func(object, id string, fields map[string]storage.Value) {
		state := org.Objects[object]
		if state.Records == nil {
			state.Records = map[storage.ID]storage.Record{}
		}
		state.Records[storage.ID(id)] = storage.Record{ID: storage.ID(id), Object: object, Fields: fields}
		org.Objects[object] = state
	}
	user := "005000000000001"
	ps := "0PS000000000001"
	cp := "0CP000000000001"
	assignment := "0Pa000000000001"
	access := "0J0000000000001"
	machine := &VM{Org: &org, executionUser: Value{Kind: ValueObject, Type: "User", Fields: map[string]Value{"Id": String(user)}}}
	check := func(want bool) {
		t.Helper()
		if got := machine.currentUserHasPermission("OwnedPermission"); got != want {
			t.Fatalf("permission=%v want %v", got, want)
		}
	}
	check(false)
	put("PermissionSet", ps, map[string]storage.Value{})
	put("CustomPermission", cp, map[string]storage.Value{"DeveloperName": storage.StringValue("OwnedPermission")})
	put("SetupEntityAccess", access, map[string]storage.Value{"ParentId": storage.IDValue(storage.ID(ps)), "SetupEntityId": storage.IDValue(storage.ID(cp))})
	check(false)
	put("PermissionSetAssignment", assignment, map[string]storage.Value{"PermissionSetId": storage.IDValue(storage.ID(ps)), "AssigneeId": storage.IDValue(storage.ID(user))})
	check(true)
	if machine.currentUserHasPermission("MissingPermission") {
		t.Fatal("missing permission granted")
	}
	delete(org.Objects["SetupEntityAccess"].Records, storage.ID(access))
	check(false)
	put("SetupEntityAccess", access, map[string]storage.Value{"ParentId": storage.IDValue(storage.ID(ps)), "SetupEntityId": storage.IDValue(storage.ID(cp))})
	check(true)
	delete(org.Objects["PermissionSetAssignment"].Records, storage.ID(assignment))
	check(false)
}
