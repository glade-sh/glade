package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestR6FlowUnknownProfileIsUnsupported(t *testing.T) {
	org := storage.NewOrgState()
	engine := NewEngine(&org)
	_, supported, handled := engine.flowGlobalDecisionCriterion(storage.WorkflowCriteriaItem{Field: "$Profile.Name", Operation: "equals", Value: ""})
	if !handled || supported {
		t.Fatalf("unknown profile treated as value: supported=%v handled=%v", supported, handled)
	}
}

func TestR6FlowRevokedPermissionDoesNotGrant(t *testing.T) {
	for _, deleted := range []string{"set", "access", "permission"} {
		t.Run(deleted, func(t *testing.T) {
			org := storage.NewOrgState()
			put := func(object string, id storage.ID, fields map[string]storage.Value) {
				state := org.Objects[object]
				if state.Records == nil {
					state.Records = map[storage.ID]storage.Record{}
				}
				r := storage.Record{ID: id, Object: object, Fields: fields}
				r.System.IsDeleted = deleted == map[string]string{"PermissionSet": "set", "SetupEntityAccess": "access", "CustomPermission": "permission"}[object]
				state.Records[id] = r
				org.Objects[object] = state
			}
			put("PermissionSet", "0PS000000000001", nil)
			put("PermissionSetAssignment", "0Pa000000000001", map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetId": storage.IDValue("0PS000000000001")})
			put("CustomPermission", "0CP000000000001", map[string]storage.Value{"DeveloperName": storage.StringValue("R6_Bypass")})
			put("SetupEntityAccess", "0J0000000000001", map[string]storage.Value{"ParentId": storage.IDValue("0PS000000000001"), "SetupEntityId": storage.IDValue("0CP000000000001")})
			engine := NewEngine(&org)
			match, supported, handled := engine.flowGlobalDecisionCriterion(storage.WorkflowCriteriaItem{Field: "$Permission.R6_Bypass", Operation: "equals", Value: "false"})
			if !handled || !supported || !match {
				t.Fatalf("revoked grant: match=%v supported=%v handled=%v", match, supported, handled)
			}
		})
	}
}

func TestR6FlowDescendingLoopRemainsUnsupported(t *testing.T) {
	step := storage.FlowStep{Kind: "loop", Loop: storage.FlowLoop{CollectionReference: "Items", IterationOrder: "Desc", Steps: []storage.FlowStep{{Kind: "customError"}}}}
	if flowStepsBeforeSaveSafeWithTargets([]storage.FlowStep{step}, false, false, nil) {
		t.Fatal("descending loop was admitted without a captured order")
	}
}
