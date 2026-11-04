package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Person updates preserve their existing family; business types reject contacts.
// Native controls: testdata/conformance/case349_default_signal/org.tsv:450 and
// testdata/conformance/case349_review/org.tsv:5-8.
func TestCase349ReviewExistingPersonContactUpdate(t *testing.T) {
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone", "PersonEmail"} {
		t.Run(field, func(t *testing.T) {
			org := testOrg()
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			storage.EnsureStandardObject(&org, "Contact")
			engine := NewEngine(&org)
			inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"LastName": storage.StringValue("Synthetic")}}})
			if len(inserted) != 1 || !inserted[0].Success {
				t.Fatalf("person insert: %#v", inserted)
			}
			value := "4155550123"
			if field == "PersonEmail" {
				value = "synthetic@example.invalid"
			}
			stored := org.Objects["Account"].Records[inserted[0].ID]
			candidate := storage.Record{Object: "Account", ID: inserted[0].ID, Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic"), field: storage.StringValue(value)}}
			if err := validateBusinessAccountContacts("Account", org.Objects["Account"].Definition, candidate, stored); err != nil {
				t.Fatalf("person contact misclassified as business: %v", err)
			}
			delete(candidate.Fields, "Name")
			result := engine.Update([]storage.Record{candidate})
			if len(result) != 1 || !result[0].Success {
				t.Fatalf("person update %s: %#v", field, result)
			}
		})
	}
}

func TestCase349ReviewBusinessContactWithPersonSignal(t *testing.T) {
	org := testOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	def := org.Objects["Account"].Definition
	def.RecordTypes = []storage.RecordTypeInfo{{ID: "012000000000001", DeveloperName: "Business", Active: true, Available: true}}
	for _, signal := range []string{"FirstName", "LastName", "PersonTitle"} {
		for _, contact := range []string{"PersonHomePhone", "PersonMobilePhone", "PersonEmail"} {
			t.Run(signal+"/"+contact, func(t *testing.T) {
				value := "4155550123"
				if contact == "PersonEmail" {
					value = "synthetic@example.invalid"
				}
				rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue("012000000000001"), signal: storage.StringValue("Synthetic"), contact: storage.StringValue(value)}}
				if err := validateBusinessAccountContacts("Account", def, rec); err == nil {
					t.Fatalf("business %s with %s accepted", contact, signal)
				}
			})
		}
	}
}

// Account custom fields are not Person Account contact fields; see Salesforce
// Account Fields and the business default rows in case349_default_signal/org.tsv:8.
func TestCase349ReviewCustomPersonPrefixDoesNotChangeFamily(t *testing.T) {
	for _, field := range []string{"PersonMemo__c", "PersonCode__c", "PersonFlag__c"} {
		t.Run(field, func(t *testing.T) {
			rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Business"), "PersonHomePhone": storage.StringValue("4155550123"), field: storage.StringValue("synthetic")}}
			if !accountUsesBusinessContactDefault("Account", rec) {
				t.Fatalf("custom field %s changed business default", field)
			}
		})
	}
}

func TestCase349ReviewCustomPersonPrefixIsNotPersonSignal(t *testing.T) {
	for _, field := range []string{"PersonMemo__c", "PersonCode__c", "PersonFlag__c"} {
		rec := storage.Record{Object: "Account", Fields: map[string]storage.Value{field: storage.StringValue("synthetic")}}
		if hasPersonAccountFieldSignal(rec) || AccountPersonSignalForBeforeTrigger(rec) {
			t.Errorf("custom field %s signaled person family", field)
		}
	}
}

// Native person-name boundary: testdata/conformance/case349_mixed/org.tsv:6,13.
func TestCase349ReviewPersonNameAndContactUpdateRejected(t *testing.T) {
	org := testOrg()
	storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
	storage.EnsureStandardObject(&org, "Contact")
	engine := NewEngine(&org)
	inserted := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"LastName": storage.StringValue("Synthetic")}}})
	if len(inserted) != 1 || !inserted[0].Success {
		t.Fatalf("person insert: %#v", inserted)
	}
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone", "PersonEmail"} {
		value := "4155550123"
		if field == "PersonEmail" {
			value = "synthetic@example.invalid"
		}
		result := engine.Update([]storage.Record{{Object: "Account", ID: inserted[0].ID, Fields: map[string]storage.Value{"Name": storage.StringValue("Synthetic"), field: storage.StringValue(value)}}})
		if len(result) != 1 || result[0].Success || result[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" {
			t.Fatalf("update with Name and %s = %#v", field, result)
		}
	}
}
