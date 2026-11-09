package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestR6FlowDirectEnginePermissionFallback(t *testing.T) {
	for _, tc := range []struct {
		name            string
		assigned, group bool
		want, supported bool
	}{
		{"unassigned", false, false, false, true},
		{"assigned", true, false, true, true},
		{"unresolved group", false, true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			put := func(object string, id storage.ID, fields map[string]storage.Value) {
				state := org.Objects[object]
				if state.Records == nil {
					state.Records = map[storage.ID]storage.Record{}
				}
				state.Records[id] = storage.Record{ID: id, Object: object, Fields: fields}
				org.Objects[object] = state
			}
			if tc.assigned {
				put("PermissionSet", "0PS000000000001", map[string]storage.Value{"CustomPermissions": storage.ListValue(storage.StringValue("R6_Bypass"))})
				put("PermissionSetAssignment", "0Pa000000000001", map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetId": storage.IDValue("0PS000000000001")})
			}
			if tc.group {
				put("PermissionSetAssignment", "0Pa000000000002", map[string]storage.Value{"AssigneeId": storage.IDValue(storage.DefaultUserID), "PermissionSetGroupId": storage.IDValue("0PG000000000001")})
			}
			engine := NewEngine(&org)
			got, supported, handled := engine.flowGlobalDecisionCriterion(storage.WorkflowCriteriaItem{Field: "$Permission.R6_Bypass", Operation: "equals", Value: "true"})
			if !handled || supported != tc.supported || (supported && got != tc.want) {
				t.Fatalf("matched=%v supported=%v handled=%v", got, supported, handled)
			}
		})
	}
}
