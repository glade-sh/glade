package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestR6FlowIncompletePermissionStateIsUnknown(t *testing.T) {
	for _, tc := range []struct {
		name                          string
		missingSet, missingPermission bool
	}{{"missingSet", true, false}, {"missingPermission", false, true}} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["PermissionSetAssignment"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0Pa000000000001": {ID: "0Pa000000000001", Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetId": storage.IDValue("0PS000000000001")}}}}
			if !tc.missingSet {
				org.Objects["PermissionSet"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0PS000000000001": {ID: "0PS000000000001"}}}
				org.Objects["SetupEntityAccess"] = storage.ObjectState{Records: map[storage.ID]storage.Record{"0J0000000000001": {ID: "0J0000000000001", Fields: map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000001"), "SetupEntityId": storage.IDValue("0CP000000000001")}}}}
			}
			engine := NewEngine(&org)
			_, supported, handled := engine.flowGlobalDecisionCriterion(storage.WorkflowCriteriaItem{Field: "$Permission.R6_Bypass", Operation: "equals", Value: "false"})
			if !handled || supported {
				t.Fatalf("incomplete permission state treated as denial: supported=%v handled=%v", supported, handled)
			}
		})
	}
}
