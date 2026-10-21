package vm

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecDatabaseUpsertFieldTokenAlias(t *testing.T) {
	for _, tc := range []struct {
		name, selector string
	}{
		{"singular_field", "Contact.Field.Email"},
		{"plural_fields_control", "Contact.Fields.Email"},
		{"direct_control", "Contact.Email"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymous(`
Schema.SObjectField selected = ` + tc.selector + `;
System.assertEquals(Contact.Email, selected);
System.assertEquals('Email', selected.getDescribe().getName());
System.assertEquals('Contact', selected.getDescribe().getSObjectType().getDescribe().getName());
Contact original = new Contact(LastName = 'Meadow', Email = 'same@example.invalid', Description = 'Before');
Database.UpsertResult inserted = Database.upsert(original, selected, true);
System.assert(inserted.isSuccess());
System.assert(inserted.isCreated());
System.assertEquals(original.Id, inserted.getId());
Contact replacement = new Contact(LastName = 'Meadow', Email = original.Email, Description = 'After');
Database.UpsertResult updated = Database.upsert(replacement, selected, true);
System.assert(updated.isSuccess());
System.assert(!updated.isCreated());
System.assertEquals(original.Id, updated.getId());
System.assertEquals(original.Id, replacement.Id);
System.assertEquals(1, [SELECT COUNT() FROM Contact]);
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

func TestSObjectFieldTokenAliasRejectionControls(t *testing.T) {
	// Preserve the existing local token-lookup boundary without claiming native
	// exception types or diagnostics for these uncaptured spellings.
	for _, selector := range []string{
		"Contact.Field.MissingField__c",
		"Contact.Fields.MissingField__c",
		"Contact.Field.Email.Name",
		"Contact.Fields.Email.Name",
		"Contcat.Field.Email",
		"Contact.Filed.Email",
		"Contact.Fieldz.Email",
	} {
		t.Run(selector, func(t *testing.T) {
			org := testDataOrg()
			storage.EnsureStandardObject(&org, "Contact")
			machine := New(nil)
			machine.SetOrg(&org)
			if token, ok := machine.lookupSObjectFieldToken(strings.Split(selector, ".")); ok {
				t.Fatalf("unexpected field token for %s: %#v", selector, token)
			}
		})
	}
}

func TestSObjectFieldTokenAliasKeepsDeclaredRelationship(t *testing.T) {
	org := testDataOrg()
	storage.EnsureStandardObject(&org, "RelationshipDomain")
	storage.EnsureStandardObject(&org, "FieldDefinition")
	machine := New(nil)
	machine.SetOrg(&org)
	// Field is an existing parent relationship on RelationshipDomain. Keep its
	// established traversal rather than interpreting it as a token qualifier.
	token, ok := machine.lookupSObjectFieldToken([]string{"RelationshipDomain", "Field", "QualifiedApiName"})
	if !ok || token.Fields["object"].Text != "FieldDefinition" || token.Fields["field"].Text != "QualifiedApiName" {
		t.Fatalf("declared relationship token changed: %#v, %v", token, ok)
	}
}
