package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecQualifiedSchemaObjectFieldTokensRemainTypedInCollections(t *testing.T) {
	program, err := CompileAnonymous(`
Set<Schema.SObjectField> fields = new Set<Schema.SObjectField>{ Schema.Account.Name, Schema.Account.ShippingCountry };
System.assertEquals(2, fields.size());
System.assert(fields.contains(Schema.Account.Name));
System.assert(fields.contains(Schema.Account.ShippingCountry));
System.assertEquals('Name', Schema.Account.Name.getDescribe().getName());
System.assertEquals('ShippingCountry', Schema.Account.ShippingCountry.getDescribe().getName());
`)
	if err != nil {
		t.Fatal(err)
	}
	machine := New(nil)
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "Account")
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
