package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestQualifiedQueryListKeepsRecordProvenance(t *testing.T) {
	program, err := CompileAnonymous(`
insert new Schema.Contact(LastName='record');
List<Schema.Contact> dynamicRows=Database.query('SELECT Id FROM Contact');
List<Schema.Contact> staticRows=[SELECT Id FROM Contact];
List<schema.contact> emptyRows=Database.query('SELECT Id FROM Contact WHERE LastName=\'absent\'');
System.assertEquals(1,dynamicRows.size()); System.assertEquals(1,staticRows.size()); System.assertEquals(0,emptyRows.size());

`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Contact")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
	// Under shadowing, check provenance preservation independently of the
	// pending native conversion outcome.
	if err := machine.RegisterClass(Class{Name: "Contact"}); err != nil {
		t.Fatal(err)
	}
	rows, err := machine.executeSOQLRowsWithExpander("SELECT Id FROM Contact", nil, machine.expandSOQLBinds, Null, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(rows) != 1 || rows[0].classInstance {
		t.Fatal("query lost record provenance")
	}

}
