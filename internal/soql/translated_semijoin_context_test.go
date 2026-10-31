package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// The native sobject_json3_qualified/org.tsv:9 captures the French semi-join.
// Reusing a parsed query must not retain a previous execution's resolved IDs.
func TestTranslatedSemiJoinResolvesPerExecution(t *testing.T) {
	org := storage.OrgState{Features: []string{"TranslationLanguage-fr"}, Metadata: storage.MetadataRegistry{ValueSetTranslations: []storage.StandardValueSetTranslation{{ValueSet: "Industry", Language: "fr", MasterLabel: "Technology", Translation: "Technologie"}}}, Objects: map[string]storage.ObjectState{
		"Account": {Definition: storage.ObjectDefinition{APIName: "Account", Fields: map[string]storage.Field{"Industry": {APIName: "Industry", Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "Technology", Label: "Technology"}}}}}, Records: map[storage.ID]storage.Record{"001000000000001": {ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Industry": storage.StringValue("Technology")}}}},
		"Contact": {Definition: storage.ObjectDefinition{APIName: "Contact", Fields: map[string]storage.Field{"LastName": {APIName: "LastName", Type: storage.FieldString}, "AccountId": {APIName: "AccountId", Type: storage.FieldReference, ReferenceTo: []string{"Account"}}}}, Records: map[storage.ID]storage.Record{"003000000000001": {ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Owned"), "AccountId": storage.IDValue("001000000000001")}}}},
	}}
	query, err := Parse("SELECT Id FROM Contact WHERE LastName = 'Owned' AND AccountId IN (SELECT Id FROM Account WHERE toLabel(Industry) = 'Technologie')")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	for _, tc := range []struct {
		lang string
		want int
	}{{"fr", 1}, {"en_US", 0}, {"fr", 1}} {
		result, err := ExecuteWithCacheAndContext(org, query, cache, ExecutionContext{Language: tc.lang})
		if err != nil {
			t.Fatal(err)
		}
		if len(result.Records) != tc.want {
			t.Errorf("language %s got %d records, want %d", tc.lang, len(result.Records), tc.want)
		}
	}
}
