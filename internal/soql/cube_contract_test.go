package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// the CUBE ceiling is independent of the ROLLUP E02 contract.
func TestCubeFieldLimitRejectsParserAndConstructedQueries(t *testing.T) {
	queryText := "SELECT COUNT(Id) total FROM Account GROUP BY CUBE(Name, Rating, Type, Industry)"
	t.Run("parser", func(t *testing.T) {
		if _, err := Parse(queryText); err == nil || err.Error() != "soql: GROUP BY CUBE must contain 3 fields or less" {
			t.Fatalf("expected CUBE field-limit error, got %v", err)
		}
	})

	t.Run("constructed_query", func(t *testing.T) {
		org := aggregateTestOrg()
		account := org.Objects["Account"]
		account.Definition.Fields["Type"] = storage.Field{
			APIName: "Type", Type: storage.FieldPicklist, Groupable: storage.BoolFlag(true),
			PicklistValues: []storage.PicklistValue{{Value: "Prospect", Label: "Prospect", Active: true}},
		}
		account.Definition.Fields["Industry"] = storage.Field{
			APIName: "Industry", Type: storage.FieldPicklist, Groupable: storage.BoolFlag(true),
			PicklistValues: []storage.PicklistValue{{Value: "Technology", Label: "Technology", Active: true}},
		}
		org.Objects["Account"] = account

		query := Query{
			Object:     "Account",
			Fields:     []string{"COUNT(Id)"},
			Aggregates: []Aggregate{{Func: "COUNT", Field: "Id"}},
			GroupMode:  "CUBE",
			GroupBy:    []string{"Name", "Rating", "Type", "Industry"},
		}
		if _, err := Execute(org, query); err == nil || err.Error() != "soql: GROUP BY CUBE must contain 3 fields or less" {
			t.Fatalf("expected constructed CUBE field-limit error, got %v", err)
		}
	})
}
