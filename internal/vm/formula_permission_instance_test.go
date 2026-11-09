package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// FormulaEval must use the same current principal as FeatureManagement.
func TestFormulaInstanceCurrentPermission(t *testing.T) {
	const uid storage.ID = "005000000000001"
	for _, field := range []string{"Permissions", "PermissionSets"} {
		t.Run(field, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{uid: {ID: uid, Fields: map[string]storage.Value{field: storage.StringValue("Bypass")}}}}
			org.Objects["Probe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Probe__c", Fields: map[string]storage.Field{}}, Records: map[storage.ID]storage.Record{}}
			machine := New(nil)
			machine.Org = &org
			machine.executionUser = Value{Kind: ValueObject, Type: "User", Fields: map[string]Value{"Id": String(string(uid)), field: String("Bypass")}}
			instance := Object("FormulaEval.FormulaInstance")
			instance.Fields["returnType"] = String("BOOLEAN")
			got, ok, err := machine.evaluateFormulaInstanceValue(instance, Object("Probe__c"), "$Permission.Bypass")
			if err != nil || !ok || got.Kind != ValueBool || !got.Bool {
				t.Fatalf("formula=%+v ok=%v err=%v", got, ok, err)
			}
		})
	}
}
