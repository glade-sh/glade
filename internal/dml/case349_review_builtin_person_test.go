package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Salesforce documents its built-in PersonAccount as a person record type:
// https://developer.salesforce.com/docs/platform/api/guide/sforce-api-guidelines-personaccounts.html
func TestCase349BuiltinPersonRecordTypeFamily(t *testing.T) {
	definition := storage.ObjectDefinition{APIName: "Account"}
	storage.EnsureStandardObjectFieldsForFeatures(&definition, []string{"PersonAccounts"})
	for _, rt := range definition.RecordTypes {
		if rt.DeveloperName != "PersonAccount" {
			continue
		}
		if !rt.PersonAccount {
			t.Errorf("built-in person type missing family flag")
		}
		record := storage.Record{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue(rt.ID)}}
		if AccountHasExplicitBusinessRecordType("Account", definition, record) {
			t.Error("built-in person type classified as business")
		}

		return
	}
	t.Fatal("built-in person record type missing")
}
