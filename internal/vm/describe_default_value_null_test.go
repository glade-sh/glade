package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// getDefaultValue() is declared to return Object. A field without a default
// yields an Object-typed null, which Integer.valueOf(Object) maps to null.
func TestDescribeDefaultValueNullIsObjectTyped(t *testing.T) {
	program, err := CompileAnonymous(`
Object raw = Account.Minutes__c.getDescribe().getDefaultValue();
System.assertEquals(null, raw);
Integer minutes = Integer.valueOf(Account.Minutes__c.getDescribe().getDefaultValue());
System.assertEquals(null, minutes);
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Minutes__c"] = storage.Field{APIName: "Minutes__c", Label: "Minutes", Type: storage.FieldDecimal, Precision: 3}
	org.Objects["Account"] = account
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
