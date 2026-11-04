package dml

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Salesforce defines business/person record type families by account kind,
// not by a substring in a configurable name:
// https://developer.salesforce.com/docs/platform/api/guide/sforce-api-guidelines-personaccounts.html
func TestCase349BusinessRecordTypeNames(t *testing.T) {
	for _, tc := range []struct {
		name, developer  string
		person, business bool
	}{
		{"Person Commerce", "Person_Commerce", false, true},
		{"Individual Business", "Individual_Business", false, true},
		{"Personalized Service", "Personalized_Service", false, true},
		{"Patient", "Patient", true, false},
		{"Business", "Business", false, true},
	} {
		definition := storage.ObjectDefinition{RecordTypes: []storage.RecordTypeInfo{{ID: "012000000000001", Name: tc.name, DeveloperName: tc.developer, PersonAccount: tc.person}}}
		record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue("012000000000001")}}
		if got := AccountHasExplicitBusinessRecordType("Account", definition, record); got != tc.business {
			t.Errorf("%s person=%v: business=%v, want %v", tc.name, tc.person, got, tc.business)
		}
	}
}
