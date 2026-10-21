package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Allocation provenance is already used for individual schema records. Keep
// it through queried-list coercion; native collision rows remain pending.
func TestQueriedSchemaListPreservesRecordProvenance(t *testing.T) {
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if err := machine.RegisterClass(Class{Name: "Contact"}); err != nil {
		t.Fatal(err)
	}
	record := Object("Contact")
	record.Fields[sobjectQueriedFieldsField] = Map()
	rows := List(record)
	rows.Type = "List<Contact>"
	for _, target := range []string{"List<Schema.Contact>", "List<schema.contact>"} {
		if _, err := machine.coerceAssignable(target, rows); err != nil {
			t.Fatalf("%s: %v", target, err)
		}
	}
	instance := Object("Contact")
	instance.classInstance = true
	instance.Fields[sobjectQueriedFieldsField] = Map()
	local := List(instance)
	local.Type = "List<Contact>"
	if _, err := machine.coerceAssignable("List<Schema.Contact>", local); err == nil {
		t.Fatal("class instance converted to schema record")
	}
	if _, err := machine.coerceAssignable("List<Schema.Account>", rows); err == nil {
		t.Fatal("Contact query converted to Account list")
	}
}
