package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native anonymous future grant: internal/apextest/testdata/conformance/r6flow_expiry/org.tsv:2,8.
func TestR6FlowFeaturePermissionDefaultUser(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{storage.DefaultUserID: {ID: storage.DefaultUserID, Object: "User", Fields: map[string]storage.Value{}}}}
	org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0PS000000000001": {ID: "0PS000000000001", Fields: map[string]storage.Value{"CustomPermissions": storage.ListValue(storage.StringValue("R6_Bypass"))}}}}
	org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0Pa000000000001": {ID: "0Pa000000000001", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetId": storage.IDValue("0PS000000000001")}}}}
	machine := New(nil)
	machine.SetOrg(&org)
	if !machine.currentUserHasPermission("R6_Bypass") {
		t.Fatal("default current user grant missing")
	}
}
