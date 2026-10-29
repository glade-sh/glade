package dml

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Lookup-filter criteria that traverse relationships (record type names,
// person-account flags, multi-hop paths).
func TestLookupFilterRelationshipPaths(t *testing.T) {
	for _, tc := range []struct {
		name, path, operation, value string
		parentType                   int // index into Account record types
		wantPass                     bool
	}{
		{"targetRecordTypeDevNameHit", "Account.RecordType.DeveloperName", "equals", "Allowed", 0, true},
		{"targetRecordTypeDevNameMiss", "Account.RecordType.DeveloperName", "equals", "Allowed", 1, false},
		{"targetRecordTypeNameNotEqual", "Account.RecordType.Name", "notEqual", "Allowed Label", 1, true},
		{"targetRecordTypeNameNotEqualHit", "Account.RecordType.Name", "notEqual", "Allowed Label", 0, false},
		{"twoHopRecordTypeId", "Contact.Account.RecordTypeId", "equals", "Allowed", 0, true},
		{"twoHopRecordTypeIdMiss", "Contact.Account.RecordTypeId", "equals", "Allowed", 1, false},
		{"twoHopRecordTypeDevName", "Contact.Account.RecordType.DeveloperName", "equals", "Allowed", 0, true},
		{"twoHopRecordTypeDevNameMiss", "Contact.Account.RecordType.DeveloperName", "equals", "Allowed", 1, false},
		{"sourceTwoHop", "$Source.Account.RecordType.DeveloperName", "equals", "Allowed", 0, true},
		{"personAccountFalse", "Account.IsPersonAccount", "equals", "False", 0, true},
		{"personAccountTrueMiss", "Account.IsPersonAccount", "equals", "True", 0, false},
		{"personAccountNotEqualTrue", "Account.IsPersonAccount", "notEqual", "True", 0, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			if strings.Contains(tc.path, "IsPersonAccount") {
				storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			}
			storage.EnsureStandardObject(&org, "Contact")
			account := org.Objects["Account"]
			account.Definition.RecordTypes = []storage.RecordTypeInfo{
				{DeveloperName: "Allowed", Name: "Allowed Label", Active: true, Available: true},
				{DeveloperName: "Other", Name: "Other Label", Active: true, Available: true},
			}
			storage.EnsureRecordTypeIDField(&account.Definition)
			org.Objects["Account"] = account
			storage.EnsureDeterministicPlatformData(&org)
			contact := org.Objects["Contact"]
			// Filter on a Contact-to-Contact lookup so the target is a Contact
			// and the Account is reached through a relationship.
			field := contact.Definition.Fields["ReportsToId"]
			field.FilteredLookupInfo = storage.FilteredLookupInfo{Active: true, FilterItems: []storage.LookupFilterItem{{Field: tc.path, Operation: tc.operation, Value: tc.value}}}
			if tc.path[:7] == "Account" {
				field = contact.Definition.Fields["AccountId"]
				field.FilteredLookupInfo = storage.FilteredLookupInfo{Active: true, FilterItems: []storage.LookupFilterItem{{Field: tc.path, Operation: tc.operation, Value: tc.value}}}
				contact.Definition.Fields["AccountId"] = field
			} else {
				contact.Definition.Fields["ReportsToId"] = field
			}
			org.Objects["Contact"] = contact
			engine := NewEngine(&org)
			rtID := org.Objects["Account"].Definition.RecordTypes[tc.parentType].ID
			acct := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("a"), "RecordTypeId": storage.IDValue(rtID)}}})
			if !acct[0].Success {
				t.Fatal(acct)
			}
			var result []Result
			if tc.path[:7] == "Account" {
				result = engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("c"), "AccountId": storage.IDValue(acct[0].ID)}}})
			} else {
				// Parent contact belongs to the account; the child reports to it.
				parent := engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("p"), "AccountId": storage.IDValue(acct[0].ID)}}})
				if !parent[0].Success {
					t.Fatal(parent)
				}
				result = engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("c"), "AccountId": storage.IDValue(acct[0].ID), "ReportsToId": storage.IDValue(parent[0].ID)}}})
			}
			if result[0].Success != tc.wantPass {
				t.Fatalf("insert = %#v; want success %v", result, tc.wantPass)
			}
		})
	}
}

// A person account's auto-created Contact reaches the Account through
// Contact.Account, so a Contact lookup filter on Contact.Account.RecordTypeId
// matches it by the person account's record type.
func TestLookupFilterPersonContactAccountRecordType(t *testing.T) {
	for _, recordType := range []int{0, 1} {
		org := testOrg()
		storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
		storage.EnsureStandardObject(&org, "Contact")
		account := org.Objects["Account"]
		// Project person record types are declared under the PersonAccount
		// metadata object and need not mention "person" in their names.
		account.Definition.RecordTypes = []storage.RecordTypeInfo{
			{DeveloperName: "PersonAccount", Name: "Person Account", Active: true, Available: true, Default: true, PersonAccount: true},
			{DeveloperName: "Provider", Name: "Provider", Active: true, Available: true, PersonAccount: true},
			{DeveloperName: "Patient", Name: "Patient", Active: true, Available: true, PersonAccount: true},
		}
		storage.EnsureRecordTypeIDField(&account.Definition)
		org.Objects["Account"] = account
		storage.EnsureDeterministicPlatformData(&org)
		contact := org.Objects["Contact"]
		field := contact.Definition.Fields["ReportsToId"]
		field.FilteredLookupInfo = storage.FilteredLookupInfo{Active: true, FilterItems: []storage.LookupFilterItem{{Field: "Contact.Account.RecordTypeId", Operation: "equals", Value: "Provider"}}}
		contact.Definition.Fields["ReportsToId"] = field
		org.Objects["Contact"] = contact
		engine := NewEngine(&org)
		person := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{
			"LastName": storage.StringValue("person"), "RecordTypeId": storage.IDValue(org.Objects["Account"].Definition.RecordTypes[recordType+1].ID),
		}}})
		if !person[0].Success {
			t.Fatal(person)
		}
		var personContact storage.ID
		for id, record := range org.Objects["Contact"].Records {
			if storage.IDsEqual(idFromStorageValue(record.Fields["AccountId"]), person[0].ID) {
				personContact = id
			}
		}
		if personContact == "" {
			t.Fatal("no person contact")
		}
		result := engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{
			"LastName": storage.StringValue("child"), "ReportsToId": storage.IDValue(personContact),
		}}})
		if want := recordType == 0; result[0].Success != want {
			t.Fatalf("recordType %d: insert = %#v; want success %v", recordType, result, want)
		}
	}
}
