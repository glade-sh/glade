package vm

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestDescribeUsesCatalogNameFieldFacet(t *testing.T) {
	// The owned standard describe catalog records nameField=true for
	// Case.CaseNumber and PermissionSetLicense.DeveloperName, and false for
	// Case.Subject and PermissionSetLicense.MasterLabel.
	program, err := CompileAnonymous(`
System.assertEquals(true, Schema.Case.CaseNumber.getDescribe().isNameField());
System.assertEquals(true, Schema.PermissionSetLicense.DeveloperName.getDescribe().isNameField());
System.assertEquals(false, Schema.Case.Subject.getDescribe().isNameField());
System.assertEquals(false, Schema.PermissionSetLicense.MasterLabel.getDescribe().isNameField());
System.assertEquals(true, Schema.Account.Name.getDescribe().isNameField());
`)
	if err != nil {
		t.Fatal(err)
	}
	org := storage.NewOrgState()
	for _, name := range []string{"Case", "PermissionSetLicense", "Account"} {
		storage.EnsureStandardObject(&org, name)
	}
	machine := New(nil)
	machine.SetOrg(&org)
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

func TestDescribeNameFieldFacetRetainsUnknownObjectFallback(t *testing.T) {
	org := storage.NewOrgState()
	org.Objects["Sample__c"] = storage.ObjectState{Definition: storage.ObjectDefinition{
		APIName: "Sample__c", Fields: map[string]storage.Field{
			"Name":             {APIName: "Name", Type: storage.FieldString},
			"DeveloperName__c": {APIName: "DeveloperName__c", Type: storage.FieldString},
		},
	}}
	machine := New(nil)
	machine.SetOrg(&org)
	program, err := CompileAnonymous(`
System.assertEquals(true, Schema.Sample__c.Name.getDescribe().isNameField());
System.assertEquals(false, Schema.Sample__c.DeveloperName__c.getDescribe().isNameField());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}

// The business-account oracle enumerates all fields at APIs 62/67/68. Legacy
// project metadata can retain person-field tokens; those must not inherit
// a person-enabled catalog's name flags in an ordinary business-account org.
func TestBusinessAccountHasExactlyOneNameField(t *testing.T) {
	raw, err := os.ReadFile("testdata/business_account_name_observations.json")
	if err != nil {
		t.Fatal(err)
	}
	var observed struct {
		Source, SourceSHA256, OrgIdSHA256 string
		Cells                             []struct {
			API, Context, RawSHA256 string
			Answer                  struct {
				PersonAccounts bool
				Flags          map[string]bool
			}
		}
	}
	if err := json.Unmarshal(raw, &observed); err != nil {
		t.Fatal(err)
	}
	if fmt.Sprintf("%x", sha256.Sum256([]byte(observed.Source))) != observed.SourceSHA256 || len(observed.Cells) != 3 || len(observed.OrgIdSHA256) != 64 {
		t.Fatal("invalid owned observation provenance")
	}
	for index, cell := range observed.Cells {
		api := cell.API
		if api != []string{"62.0", "67.0", "68.0"}[index] || cell.Context != "anonymous" || len(cell.RawSHA256) != 64 || cell.Answer.PersonAccounts || len(cell.Answer.Flags) != 1 || !cell.Answer.Flags["Name"] {
			t.Fatal("changed business-account oracle")
		}
		t.Run(api, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(`
Integer count = 0;
Schema.SObjectField selected;
for (Schema.SObjectField token : Account.SObjectType.getDescribe().fields.getMap().values()) {
    if (token.getDescribe().isNameField()) {
        count++;
        selected = token;
        System.assertEquals('Name', token.getDescribe().getName());
    }
}
System.assertEquals(1, count);
Account caller = new Account(Name = 'Owned business');
insert caller;
Account queried = [SELECT Name FROM Account WHERE Id = :caller.Id];
System.assertEquals('Owned business', queried.get(selected));
`, CompileOptions{APIVersion: api})
			if err != nil {
				t.Fatal(err)
			}
			org := storage.NewOrgState()
			storage.EnsureStandardObject(&org, "Account")
			// A standard-object metadata delta can omit record-type metadata.
			account := org.Objects["Account"]
			account.Definition.RecordTypes = nil
			// Legacy standard metadata may retain component tokens without
			// declaring the PersonAccounts org feature.
			account.Definition.Fields["FirstName"] = storage.Field{APIName: "FirstName", Type: storage.FieldString, CompoundFieldName: "Name"}
			account.Definition.Fields["LastName"] = storage.Field{APIName: "LastName", Type: storage.FieldString, CompoundFieldName: "Name"}
			org.Objects["Account"] = account
			machine := New(nil)
			machine.SetOrg(&org)
			if _, err := machine.Execute(program); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestPersonAccountRetainsCatalogNameFieldFacets(t *testing.T) {
	org := storage.NewOrgState()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	machine := New(nil)
	machine.SetOrg(&org)
	program, err := CompileAnonymous(`
System.assertEquals(true, Account.Name.getDescribe().isNameField());
System.assertEquals(true, Account.FirstName.getDescribe().isNameField());
System.assertEquals(true, Account.LastName.getDescribe().isNameField());
`)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := machine.Execute(program); err != nil {
		t.Fatal(err)
	}
}
