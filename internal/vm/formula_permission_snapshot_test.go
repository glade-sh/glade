package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Synthetic execution-user fields may be supplied independently of the stored
// user; Apex and the validation formula must still see the same principal.
func TestFormulaPermissionExecutionUserSnapshot(t *testing.T) {
	const uid storage.ID = "005000000000001"
	for _, field := range []string{"Permissions", "PermissionSets"} {
		t.Run(field, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{uid: {ID: uid, Fields: map[string]storage.Value{}}}}
			org.Objects["Probe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Probe__c", KeyPrefix: "a01", Fields: map[string]storage.Field{}, ValidationRules: []storage.ValidationRule{{Name: "Guard", Active: true, ErrorConditionFormula: "NOT($Permission.Bypass)", ErrorMessage: "denied"}}}, Records: map[storage.ID]storage.Record{}}
			machine := New(nil)
			machine.Org = &org
			machine.executionUser = Value{Kind: ValueObject, Type: "User", Fields: map[string]Value{"Id": String(string(uid)), field: String("Bypass")}}
			if !machine.currentUserHasPermission("Bypass") {
				t.Fatal("Apex did not see the execution-user grant")
			}
			engine := machine.newDMLEngine(&Result{})
			result := engine.Insert([]storage.Record{{Object: "Probe__c", Fields: map[string]storage.Value{}}})
			if len(result) != 1 || !result[0].Success {
				t.Fatalf("formula disagrees with Apex: %+v", result)
			}
		})
	}
}
