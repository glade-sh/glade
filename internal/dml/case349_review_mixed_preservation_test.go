package dml

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Explicit business types reject person contact fields under ruling 38b.
// Native mixed controls: case349_review2/org.tsv:2-5,11-14 (API 62/67).
func TestCase349MixedFullPersonNameRejectsBusinessPersonFields(t *testing.T) {
	for _, field := range []string{"PersonHomePhone", "PersonMobilePhone"} {
		t.Run(field, func(t *testing.T) {
			org := storage.NewOrgState()
			storage.EnsureDeterministicPlatformData(&org)
			storage.ApplyOrgShape(&org, []string{"PersonAccounts"})
			account := org.Objects["Account"]
			account.Definition.RecordTypes = append(account.Definition.RecordTypes, storage.RecordTypeInfo{ID: "012000000000ORG", DeveloperName: "Organization", Active: true, Available: true})
			org.Objects["Account"] = account
			storage.EnsureDeterministicPlatformData(&org)
			engine := NewEngine(&org)
			result := engine.Insert([]storage.Record{{Object: "Account", Fields: map[string]storage.Value{"RecordTypeId": storage.IDValue("012000000000ORG"), "Name": storage.StringValue("Synthetic"), "FirstName": storage.StringValue("Synthetic"), "LastName": storage.StringValue("Person"), field: storage.StringValue("4155550123")}}})
			if len(result) != 1 || result[0].Success || result[0].StatusCode != "INVALID_FIELD_FOR_INSERT_UPDATE" {
				t.Fatalf("business person field rejection: %#v", result)
			}
		})
	}
}
