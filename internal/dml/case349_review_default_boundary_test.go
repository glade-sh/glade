package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native omitted-type contact control confirms a business default:
// case349_default_signal/org.tsv:8. Other defaults retain the prior person path.
func TestCase349ContactDefaultRequiresKnownBusinessDefault(t *testing.T) {
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone", "PersonEmail"} {
		for _, scenario := range []string{"person", "unknown"} {
			t.Run(field+"/"+scenario, func(t *testing.T) {
				types := []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Business", Active: true, Available: true}, {ID: "012000000000002", DeveloperName: "Individual", Active: true, Available: true, PersonAccount: true}}
				if scenario == "person" {
					types[1].Default = true
				}
				rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic"), field: storage.StringValue("synthetic")}}
				selected, ok := defaultRecordTypeForRecord("Account", types, rec)
				if !ok || !selected.PersonAccount {
					t.Fatalf("selected %#v, want the prior person route", selected)
				}
			})
		}
	}
}

// Seeding supplies a default in each family. The captured business default
// remains business in this shape: case349_default_signal/org.tsv:17.
func TestCase349ContactDefaultWithBothFamilyDefaults(t *testing.T) {
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone", "PersonEmail"} {
		types := []storage.RecordTypeInfo{
			{ID: "012000000000001", Active: true, Available: true, Default: true},
			{ID: "012000000000002", Active: true, Available: true, Default: true, PersonAccount: true},
		}
		rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic"), field: storage.StringValue("synthetic")}}
		selected, ok := defaultRecordTypeForRecord("Account", types, rec)
		if !ok || selected.PersonAccount {
			t.Errorf("%s: selected %#v, want captured business default", field, selected)
		}
	}
}

// Native person-profile controls reject populated contact fields:
// case349_profile_boundary/org.tsv:33,35,37 and API 67 twins.
func TestCase349ContactPersonProfileDirectDML(t *testing.T) {
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone", "PersonEmail"} {
		org := testOrg()
		storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
		storage.EnsureDeterministicPlatformData(&org)
		account := org.Objects["Account"]
		account.Definition.RecordTypes = []storage.RecordTypeInfo{
			{ID: "012000000000001", DeveloperName: "Business", Active: true, Available: true, Default: true},
			{ID: "012000000000002", DeveloperName: "Individual", Active: true, Available: true, PersonAccount: true},
		}
		org.Objects["Account"] = account
		org.Metadata.AccountRecordTypeDefaults = []storage.AccountRecordTypeDefault{{ProfileID: storage.DefaultProfileID, DeveloperName: "Individual", PersonAccount: true}}
		value := "4155550123"
		if field == "PersonEmail" {
			value = "synthetic@example.invalid"
		}
		engine := NewEngine(&org)
		result := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic"), field: storage.StringValue(value)}}})
		if len(result) != 1 || result[0].Success || result[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" {
			t.Errorf("%s: got %#v, want native business rejection", field, result)
		}
	}
}

// Native person-profile controls select the business family for these inputs:
// case349_profile_boundary/org.tsv:32-37.
func TestCase349ContactDefaultPreservesPersonProfileRoute(t *testing.T) {
	org := testOrg()
	storage.EnsureDeterministicPlatformData(&org)
	types := []storage.RecordTypeInfo{
		{ID: "012000000000001", DeveloperName: "Business", Active: true, Available: true, Default: true},
		{ID: "012000000000002", DeveloperName: "Individual", Active: true, Available: true, PersonAccount: true},
	}
	org.Metadata.AccountRecordTypeDefaults = []storage.AccountRecordTypeDefault{
		{ProfileID: storage.DefaultProfileID, DeveloperName: "Business"},
		{ProfileID: storage.DefaultProfileID, DeveloperName: "Individual", PersonAccount: true},
	}
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone", "PersonEmail"} {
		rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic"), field: storage.StringValue("synthetic")}}
		selected, ok := AccountDefaultRecordType(&org, storage.DefaultUserID, storage.ObjectDefinition{RecordTypes: types}, rec)
		if !ok || selected.PersonAccount {
			t.Errorf("%s: selected %#v, want native business route", field, selected)
		}
	}
}
