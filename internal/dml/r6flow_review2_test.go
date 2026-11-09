package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestR6FlowHostUnresolvedPermissionGroupFailsClosed(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0Pa000000000001": {ID: "0Pa000000000001", Object: "PermissionSetAssignment", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetGroupId": storage.IDValue("0PG000000000001")}}}}
	engine := NewEngine(&org)
	engine.UserHasCustomPermission = func(string) bool { return false }
	_, supported, handled := engine.flowGlobalDecisionCriterion(storage.WorkflowCriteriaItem{Field: "$Permission.R6_Bypass", Operation: "equals", Value: "false"})
	if !handled || supported {
		t.Fatalf("unresolved group treated as denial: supported=%v handled=%v", supported, handled)
	}
}
