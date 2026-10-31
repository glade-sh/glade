package vm

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
)

// Preserve shipping behavior for uncaptured list upserts, including a
// singleton list. These are local controls, not Salesforce parity claims.
func TestUpsertDuplicateSingletonListPreservesShippingBehavior(t *testing.T) {
	loaded, err := project.Load(filepath.Join("../apextest/testdata/conformance/dml_policies", "project"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		for _, route := range []string{"statement", "database"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				org := storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
				storage.EnsureStandardObject(&org, "Contact")
				contact := org.Objects["Contact"]
				contact.Definition.Fields["RouteKey__c"] = storage.Field{APIName: "RouteKey__c", Type: storage.FieldString, ExternalID: true}
				org.Objects["Contact"] = contact
				installDMLPolicyAutomation(t, &org, loaded)
				body := `insert new Contact(LastName='Block', Email='duplicate@example.invalid', RouteKey__c='seed');
                    List<Contact> rows = new List<Contact>{new Contact(LastName='Block', Email='duplicate@example.invalid', RouteKey__c='new')};`
				if route == "statement" {
					body += `upsert rows RouteKey__c;`
				} else {
					body += `List<Database.UpsertResult> saved = Database.upsert(rows, Contact.RouteKey__c, false); System.assertEquals(true, saved[0].isSuccess());`
				}
				body += `System.assertEquals(2, [SELECT COUNT() FROM Contact]);`
				program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
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
}

// Partial DML can filter a list to one survivor before the engine sees it.
// The original list shape must remain on the uncaptured shipping path.
func TestUpsertDuplicateFilteredListPreservesShippingBehavior(t *testing.T) {
	loaded, err := project.Load(filepath.Join("../apextest/testdata/conformance/dml_policies", "project"))
	if err != nil {
		t.Fatal(err)
	}
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			org := storage.OrgState{APIVersion: api, Objects: map[string]storage.ObjectState{}}
			storage.EnsureStandardObject(&org, "Contact")
			contact := org.Objects["Contact"]
			contact.Definition.Fields["RouteKey__c"] = storage.Field{APIName: "RouteKey__c", Type: storage.FieldString, ExternalID: true}
			org.Objects["Contact"] = contact
			installDMLPolicyAutomation(t, &org, loaded)
			body := `insert new Contact(LastName='Block', Email='duplicate@example.invalid', RouteKey__c='seed');
                List<Contact> rows = new List<Contact>{new Contact(LastName='Block', Email='duplicate@example.invalid', RouteKey__c='new'), new Contact(LastName='Block', Email='different@example.invalid')};
                List<Database.UpsertResult> saved = Database.upsert(rows, Contact.RouteKey__c, false);
                System.assertEquals(true, saved[0].isSuccess());
                System.assertEquals(false, saved[1].isSuccess());
                System.assertEquals(2, [SELECT COUNT() FROM Contact]);`
			program, err := CompileAnonymousWithOptions(body, CompileOptions{APIVersion: api})
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
