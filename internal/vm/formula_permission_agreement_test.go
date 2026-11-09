package vm

import (
	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Synthetic stored grant representations must have the same effect in the VM
// and the DML formula evaluator.
func TestFormulaAndApexPermissionSyntheticGrantAgreement(t *testing.T) {
	const uid storage.ID = "005000000000001"
	const sid storage.ID = "0PS000000000001"
	for _, tc := range []struct{ object, field string }{
		{"User", "Permissions"}, {"User", "PermissionSets"}, {"PermissionSet", "Permissions"}, {"PermissionSet", "PermissionSets"}, {"PermissionSet", "CustomPermissions"},
	} {
		t.Run(tc.object+"/"+tc.field, func(t *testing.T) {
			userFields := map[string]storage.Value{}
			setFields := map[string]storage.Value{}
			vmFields := map[string]Value{"Id": String(string(uid))}
			if tc.object == "User" {
				userFields[tc.field] = storage.StringValue("Bypass")
				vmFields[tc.field] = String("Bypass")
			} else {
				setFields[tc.field] = storage.StringValue("Bypass")
			}
			org := storage.OrgState{Objects: map[string]storage.ObjectState{
				"User":                    {Records: map[storage.ID]storage.Record{uid: {ID: uid, Fields: userFields}}},
				"PermissionSet":           {Records: map[storage.ID]storage.Record{sid: {ID: sid, Fields: setFields}}},
				"PermissionSetAssignment": {Records: map[storage.ID]storage.Record{"0Pa000000000001": {Fields: map[string]storage.Value{"AssigneeId": storage.IDValue(uid), "PermissionSetId": storage.IDValue(sid)}}}},
			}}
			machine := &VM{Org: &org, executionUser: Value{Kind: ValueObject, Type: "User", Fields: vmFields}}
			apex := machine.currentUserHasPermission("Bypass")
			result, _, ok := dml.EvaluateRecordFormulaValueInOrgWithOptions("$Permission.Bypass", storage.Field{Type: storage.FieldBoolean}, &org, storage.ObjectDefinition{}, storage.Record{}, dml.FormulaEvaluationOptions{UserID: uid})
			if !ok || !apex || result.Kind != storage.ValueBoolean || result.Boolean != apex {
				t.Fatalf("apex=%v formula=%+v ok=%v", apex, result, ok)
			}
		})
	}
}
