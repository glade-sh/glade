package soql

import (
	"github.com/glade-sh/glade/internal/storage"
	"testing"
)

// Native sobject_json3_qualified/org.tsv:5-7 captures translated status predicates.
func TestTranslatedPredicateStatusSets(t *testing.T) {
	for _, tc := range []struct{ object, field, set string }{{"Task", "Status", "TaskStatus"}, {"Order", "Status", "OrderStatus"}, {"Contract", "Status", "ContractStatus"}} {
		t.Run(tc.object, func(t *testing.T) {
			org := storage.OrgState{Features: []string{"TranslationLanguage-fr"}, Metadata: storage.MetadataRegistry{ValueSetTranslations: []storage.StandardValueSetTranslation{{ValueSet: tc.set, Language: "fr", MasterLabel: "Original", Translation: "Traduit"}}}}
			def := storage.ObjectDefinition{APIName: tc.object, Fields: map[string]storage.Field{tc.field: {APIName: tc.field, Type: storage.FieldPicklist, PicklistValues: []storage.PicklistValue{{Value: "source", Label: "Original"}}}}}
			record := storage.Record{Object: tc.object, Fields: map[string]storage.Value{tc.field: storage.StringValue("source")}}
			condition := &Condition{Field: "toLabel(" + tc.field + ")", Op: "=", Value: storage.StringValue("Traduit")}
			if !matchesWithContext(org, def, record, condition, ExecutionContext{Language: "fr"}) {
				t.Fatal("translated label should match")
			}
		})
	}
}
