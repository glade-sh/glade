package vm

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native field token outcomes: ../sema/testdata/schema_shorthand/org.tsv:2-5,11-14.
func TestSObjectTypeFieldShorthandNative(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := testDataOrg()
			org.APIVersion = api
			org.Objects["Probe__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{APIName: "Probe__c", Fields: map[string]storage.Field{"Name": {APIName: "Name", Type: storage.FieldString}}}, Records: map[storage.ID]storage.Record{}}
			// The Schema.SObjectType.Account.Name assertion below preserves prior
			// local describe-property behavior; no native row is claimed for it.
			program, err := CompileAnonymousWithOptions(`
System.assertEquals('Name', Account.SObjectType.Name.getDescribe().getName());
System.assertEquals('Id', Account.SObjectType.Id.getDescribe().getName());
System.assertEquals('Name', Probe__c.SObjectType.Name.getDescribe().getName());
System.assertEquals(Account.Name, Account.SObjectType.Name);
System.assertEquals('Account', Schema.SObjectType.Account.Name);
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}
