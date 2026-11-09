package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestFormulaPermissionNestedAndBoxedSnapshots(t *testing.T) {
	const uid storage.ID = "005000000000001"
	boxed := Object("String")
	boxed.Fields["value"] = String("Bypass")
	for _, tc := range []struct {
		name  string
		value Value
	}{{"nested", List(List(String("Bypass")))}, {"boxed", List(boxed)}} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.NewOrgState()
			org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{uid: {ID: uid, Fields: map[string]storage.Value{}}}}
			org.Objects["Probe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Probe__c", KeyPrefix: "a01", Fields: map[string]storage.Field{}, ValidationRules: []storage.ValidationRule{{Name: "Guard", Active: true, ErrorConditionFormula: "NOT($Permission.Bypass)", ErrorMessage: "denied"}}}, Records: map[storage.ID]storage.Record{}}
			machine := New(nil)
			machine.Org = &org
			machine.executionUser = Value{Kind: ValueObject, Type: "User", Fields: map[string]Value{"Id": String(string(uid)), "Permissions": tc.value}}
			if !machine.currentUserHasPermission("Bypass") {
				t.Fatal("snapshot grant not visible in Apex")
			}
			engine := machine.newDMLEngine(&Result{})
			result := engine.Insert([]storage.Record{{Object: "Probe__c", Fields: map[string]storage.Value{}}})
			if len(result) != 1 || !result[0].Success {
				t.Fatalf("formula disagrees with Apex: %+v", result)
			}
		})
	}
}
