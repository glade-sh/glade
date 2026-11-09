package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestFormulaPermissionCalculatedReadPrincipal(t *testing.T) {
	const uid storage.ID = "005000000000001"
	org := storage.NewOrgState()
	org.Objects["User"] = storage.ObjectState{Records: map[storage.ID]storage.Record{uid: {ID: uid, Fields: map[string]storage.Value{"Permissions": storage.StringValue("Bypass")}}}}
	field := storage.Field{APIName: "Granted__c", Type: storage.FieldCalculated, DisplayType: "BOOLEAN", Formula: "$Permission.Bypass"}
	def := storage.ObjectDefinition{APIName: "Probe__c", Fields: map[string]storage.Field{"Granted__c": field}}
	org.Objects["Probe__c"] = storage.ObjectState{Definition: def, Records: map[storage.ID]storage.Record{}}
	machine := New(nil)
	machine.Org = &org
	machine.executionUser = Value{Kind: ValueObject, Type: "User", Fields: map[string]Value{"Id": String(string(uid))}}
	record := Object("Probe__c")
	got, ok := machine.missingSObjectFieldValue(record, "Granted__c")
	if !ok || got.Kind != ValueBool || !got.Bool {
		t.Errorf("SObject read: %+v ok=%v", got, ok)
	}
	stored := storage.Record{Object: "Probe__c", Fields: map[string]storage.Value{}}
	machine.populateCalculatedTriggerFields(&stored)
	if got := stored.Fields["Granted__c"]; got.Kind != storage.ValueBoolean || !got.Boolean {
		t.Errorf("trigger field: %+v", got)
	}
}
