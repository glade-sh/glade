package vm

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The owned upsert_fields observations establish Contact.Fields.Email for a
// statement. The documented statement grammar also accepts an SObject list:
// https://trailhead.salesforce.com/content/learn/modules/apex_database/apex_database_dml
func TestExecUpsertStatementFieldsSelector(t *testing.T) {
	for _, tc := range []struct {
		name, operation, assertion string
	}{
		{
			name:      "scalar_fields",
			operation: `upsert replacement Contact.Fields.Email;`,
			assertion: `System.assertEquals(original.Id, replacement.Id); System.assertEquals(1, [SELECT COUNT() FROM Contact]);`,
		},
		{
			name: "list_fields",
			operation: `List<Contact> input = new List<Contact>{replacement, new Contact(LastName = 'Meadow New', Email = 'new@example.invalid')};
upsert input Contact.Fields.Email;`,
			assertion: `System.assertEquals(original.Id, input[0].Id); System.assertNotEquals(null, input[1].Id); System.assertEquals(2, [SELECT COUNT() FROM Contact]);`,
		},
		{
			name:      "unqualified_statement_control",
			operation: `upsert replacement Email;`,
			assertion: `System.assertEquals(original.Id, replacement.Id); System.assertEquals(1, [SELECT COUNT() FROM Contact]);`,
		},
		{
			name: "database_direct_control",
			operation: `Database.UpsertResult outcome = Database.upsert(replacement, Contact.Email, true);
System.assert(outcome.isSuccess()); System.assert(!outcome.isCreated()); System.assertEquals(original.Id, outcome.getId());`,
			assertion: `System.assertEquals(1, [SELECT COUNT() FROM Contact]);`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(`
Contact original = new Contact(LastName = 'Meadow', Email = 'same@example.invalid', Description = 'Before');
insert original;
Contact replacement = new Contact(LastName = 'Meadow', Email = original.Email, Description = 'After');
` + tc.operation + tc.assertion + `
System.assertEquals('After', [SELECT Description FROM Contact WHERE Id = :original.Id].Description);
`)
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			storage.EnsureStandardObject(&org, "Contact")
			storage.EnsureDeterministicPlatformData(&org)
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestExecUpsertStatementFieldsSelectorRejectionControls(t *testing.T) {
	// These checks preserve the existing local rejection boundary. They do
	// not claim a newly captured native error type or diagnostic string.
	for _, tc := range []struct {
		name, selector string
	}{
		{"wrong_owner", "Lead.Fields.Email"},
		{"nonexternal_field", "Contact.Fields.Description"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(`Contact row = new Contact(LastName = 'Meadow', Email = 'same@example.invalid'); upsert row ` + tc.selector + `;`)
			if err != nil {
				t.Fatal(err)
			}
			org := testDataOrg()
			storage.EnsureStandardObject(&org, "Contact")
			storage.EnsureStandardObject(&org, "Lead")
			storage.EnsureDeterministicPlatformData(&org)
			machine := New(nil)
			machine.SetOrg(&org)
			_, err = machine.Execute(program)
			var runtimeErr *RuntimeError
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "System.SObjectException" {
				t.Fatalf("expected preserved SObjectException rejection, got %v", err)
			}
		})
	}
}
