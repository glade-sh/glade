package soql

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func TestExecuteIdShapedTextEqualityUsesDeclaredFieldType(t *testing.T) {
	// Native R013/R015/R017/R019 reject the different-width text match;
	// R047 accepts the exact text. Id/reference comparison stays unchanged.
	const short = "001000000000001"
	const long = short + "AAA"
	for _, fieldType := range []storage.FieldType{storage.FieldString, storage.FieldID, storage.FieldReference} {
		for _, indexed := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/indexed=%t", fieldType, indexed), func(t *testing.T) {
				definition := storage.ObjectDefinition{
					APIName: "Widget__c",
					Fields: map[string]storage.Field{
						"Token__c": {APIName: "Token__c", Type: fieldType, ReferenceTo: []string{"Widget__c"}},
					},
				}
				if indexed {
					definition.Indexes = []storage.IndexDefinition{{Name: "Widget.Token", Object: "Widget__c", Fields: []string{"Token__c"}}}
				}
				org := storage.NewOrgState()
				org.Objects["Widget__c"] = storage.ObjectState{
					Definition: definition,
					Records: map[storage.ID]storage.Record{
						short: {ID: short, Object: "Widget__c", Fields: map[string]storage.Value{"Token__c": storage.StringValue(short)}},
					},
				}
				storage.RebuildIndexes(&org)
				for _, predicate := range []struct {
					text             string
					textRows, idRows int
				}{
					{"= '" + long + "'", 0, 1},
					{"IN ('" + long + "')", 0, 1},
					{"!= '" + long + "'", 1, 0},
					{"NOT IN ('" + long + "')", 1, 0},
					{"= '" + short + "'", 1, 1},
					{"IN ('" + short + "')", 1, 1},
				} {
					query := "SELECT Id FROM Widget__c WHERE Token__c " + predicate.text
					result, err := ParseAndExecute(org, query)
					if err != nil {
						t.Fatalf("%s: %v", query, err)
					}
					want := predicate.idRows
					if fieldType == storage.FieldString {
						want = predicate.textRows
					}
					if result.Rows != want {
						t.Errorf("%s: got %d rows, want %d", query, result.Rows, want)
					}
				}
			})
		}
	}
}
