package vm

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

func TestTriggerSummaryReadPreservesSnapshot(t *testing.T) {
	// Salesforce trigger context documentation defines old/oldMap as the
	// versions before the update, including populated summary values.
	org := summarySideEffectTestOrg("Line__c")
	parent := org.Objects["Invoice__c"]
	field := parent.Definition.Fields["Total__c"]
	field.SummaryOperation = "sum"
	parent.Definition.Fields["Total__c"] = field
	org.Objects["Invoice__c"] = parent
	child := org.Objects["Line__c"]
	child.Records = map[storage.ID]storage.Record{"a01000000000001": {ID: "a01000000000001", Object: "Line__c", Fields: map[string]storage.Value{
		"Invoice__c": storage.IDValue("a02000000000001"), "Amount__c": storage.DecimalValue("7"),
	}}}
	org.Objects["Line__c"] = child
	machine := New(nil)
	machine.SetOrg(&org)
	// Missing-snapshot defaults remain pending; populated old values are documented.
	old := Object("Invoice__c")
	old.Fields["Id"] = String("a02000000000001")
	old.Fields["Total__c"] = Int(0)
	markTriggerSObject(&old)
	machine.Globals["oldRow"] = old
	program, err := CompileAnonymous(`System.assertEquals(0,oldRow.Total__c); System.assertEquals(0,oldRow.get('Total__c'));`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
