package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native explicit-person rows in testdata/conformance/lookup_filter_followups/org.tsv
// preserve Named_Person, Named_Individual and Care. Legacy metadata can identify
// a person record type by its name without carrying the newer PersonAccount flag.
func TestPersonAccountExplicitNameOnlyFollowups(t *testing.T) {
	for _, tc := range []struct{ name, developer, label string }{
		{"personLabel", "Care", "Care Person"},
		{"individualDeveloper", "Individual_Care", "Care"},
		{"individualLabel", "Member", "Individual Member"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureStandardObject(&org, "Contact")
			account := org.Objects["Account"]
			account.Definition.RecordTypes = []storage.RecordTypeInfo{
				{DeveloperName: "PersonAccount", Name: "Person Account", Active: true, Available: true, Default: true, PersonAccount: true},
				{DeveloperName: tc.developer, Name: tc.label, Active: true, Available: true},
			}
			storage.EnsureRecordTypeIDField(&account.Definition)
			org.Objects["Account"] = account
			storage.EnsureDeterministicPlatformData(&org)
			contact := org.Objects["Contact"]
			field := contact.Definition.Fields["ReportsToId"]
			field.FilteredLookupInfo = storage.FilteredLookupInfo{Active: true, FilterItems: []storage.LookupFilterItem{{
				Field: "Contact.Account.RecordTypeId", Operation: "equals", Value: tc.developer,
			}}}
			contact.Definition.Fields["ReportsToId"] = field
			org.Objects["Contact"] = contact
			engine := NewEngine(&org)
			for _, index := range []int{1, 0} {
				rt := org.Objects["Account"].Definition.RecordTypes[index]
				if index == 1 && rt.PersonAccount {
					t.Fatal("fixture must exercise an unflagged person record type")
				}
				result := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{
					"LastName": storage.StringValue("Owned"), "RecordTypeId": storage.IDValue(rt.ID),
				}}})
				if len(result) != 1 || !result[0].Success {
					t.Fatalf("person insert = %#v", result)
				}
				stored := org.Objects["Account"].Records[result[0].ID]
				if actual := idFromStorageValue(stored.Fields["RecordTypeId"]); !storage.IDsEqual(actual, rt.ID) {
					t.Fatalf("explicit record type replaced: got %s, want %s", actual, rt.ID)
				}
				personContact := idFromStorageValue(stored.Fields["PersonContactId"])
				linked, ok := org.Objects["Contact"].Records[personContact]
				if !ok || !storage.IDsEqual(idFromStorageValue(linked.Fields["AccountId"]), stored.ID) {
					t.Fatalf("missing linked person contact: account=%#v contact=%#v", stored, linked)
				}
				filtered := engine.Insert([]storage.Record{{Object: "Contact", Fields: map[string]storage.Value{
					"LastName": storage.StringValue("Child"), "ReportsToId": storage.IDValue(personContact),
				}}})
				if len(filtered) != 1 || filtered[0].Success != (index == 1) {
					t.Fatalf("lookup for record type %s = %#v; want success %v", rt.DeveloperName, filtered, index == 1)
				}
				if index == 0 && (len(filtered[0].Errors) != 1 || filtered[0].Errors[0].StatusCode != "FIELD_FILTER_VALIDATION_EXCEPTION") {
					t.Fatalf("lookup rejection = %#v", filtered)
				}
			}
		})
	}
}
