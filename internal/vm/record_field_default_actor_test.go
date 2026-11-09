package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func recordFieldDefaultActorMachine(t *testing.T, user Value) (*VM, storage.ObjectDefinition, storage.Record) {
	t.Helper()
	definition := storage.ObjectDefinition{APIName: "Probe_Item__c", Fields: map[string]storage.Field{
		"Detail__c":  {APIName: "Detail__c", Type: storage.FieldString},
		"Granted__c": {APIName: "Granted__c", Type: storage.FieldBoolean, DefaultValue: "$Permission.Probe_Grant"},
	}}
	org := &storage.OrgState{Objects: map[string]storage.ObjectState{"Probe_Item__c": {Definition: definition, Records: map[storage.ID]storage.Record{}}}}
	machine := &VM{Org: org, executionUser: user}
	record := storage.Record{Object: "Probe_Item__c", Fields: map[string]storage.Value{"Name": storage.StringValue("S")}}
	return machine, definition, record
}

// The fixture permission snapshot still reaches a default formula that reads
// $Permission, while the common field without a default never builds it.
func TestDefaultValueForRecordFieldResolvesActorOnlyForFormulaDefaults(t *testing.T) {
	plain := Value{Kind: ValueObject, Type: "User", Fields: map[string]Value{"Id": String("005000000000001")}}
	granted := Value{Kind: ValueObject, Type: "User", Fields: map[string]Value{
		"Id":          String("005000000000001"),
		"Permissions": {Kind: ValueList, List: []Value{String("Probe_Grant")}},
	}}

	machine, definition, record := recordFieldDefaultActorMachine(t, granted)
	if fields := machine.currentUserPermissionFields(); len(fields) != 1 {
		t.Fatalf("fixture permission fields = %v, want the Permissions snapshot", fields)
	}
	value, ok := machine.defaultValueForRecordField(definition, record, definition.Fields["Granted__c"])
	if !ok || value.Kind != storage.ValueBoolean || !value.Boolean {
		t.Fatalf("granted default = %#v, %v; want true", value, ok)
	}
	plainMachine, _, _ := recordFieldDefaultActorMachine(t, plain)
	value, ok = plainMachine.defaultValueForRecordField(definition, record, definition.Fields["Granted__c"])
	if !ok || value.Kind != storage.ValueBoolean || value.Boolean {
		t.Fatalf("plain default = %#v, %v; want false", value, ok)
	}

	noDefault := definition.Fields["Detail__c"]
	measure := func(machine *VM) float64 {
		return testing.AllocsPerRun(50, func() {
			if _, ok := machine.defaultValueForRecordField(definition, record, noDefault); ok {
				t.Fatal("field without a default produced a value")
			}
		})
	}
	withSnapshot, without := measure(machine), measure(plainMachine)
	t.Logf("allocations: fixture snapshot=%.0f plain=%.0f", withSnapshot, without)
	if withSnapshot != without {
		t.Fatalf("a field without a default resolved the permission snapshot: %.0f allocations vs %.0f", withSnapshot, without)
	}
}
