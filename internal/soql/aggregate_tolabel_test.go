package soql

import (
	"errors"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native flow_tolabel_residuals (API 62.0 and 67.0): in an aggregate query an
// aliased toLabel(field) needs no grouping and is the label of the group's MIN
// non-null value; an unaliased toLabel(field) needs the field grouped and is
// exposed under it; otherwise a catchable QueryException names the field. A
// non-aggregate toLabel query keeps its row labels.
func TestAggregateToLabelRules(t *testing.T) {
	org := aggregateTestOrg()
	account := org.Objects["Account"]
	account.Definition.Fields["Rating"] = storage.Field{
		APIName: "Rating",
		Type:    storage.FieldPicklist,
		PicklistValues: []storage.PicklistValue{
			{Value: "Hot", Label: "Hot Label"},
			{Value: "Warm", Label: "Warm Label"},
		},
	}
	org.Objects["Account"] = account

	run := func(t *testing.T, query string) []storage.Record {
		t.Helper()
		result, err := ParseAndExecute(org, query)
		if err != nil {
			t.Fatalf("%s: %v", query, err)
		}
		return result.Records
	}

	t.Run("aliased_grouped", func(t *testing.T) {
		got := map[string]int{}
		for _, record := range run(t, "SELECT toLabel(Rating) lbl, COUNT(Id) c FROM Account GROUP BY Rating") {
			got[record.Fields["lbl"].String] = int(record.Fields["c"].Integer)
		}
		if len(got) != 2 || got["Hot Label"] != 2 || got["Warm Label"] != 1 {
			t.Fatalf("groups = %v", got)
		}
	})
	t.Run("aliased_ungrouped_min", func(t *testing.T) {
		records := run(t, "SELECT toLabel(Rating) lbl, COUNT(Id) c FROM Account")
		if len(records) != 1 || records[0].Fields["lbl"].String != "Hot Label" || records[0].Fields["c"].Integer != 3 {
			t.Fatalf("records = %#v", records)
		}
	})
	t.Run("unaliased_grouped", func(t *testing.T) {
		got := map[string]bool{}
		for _, record := range run(t, "SELECT toLabel(Rating), COUNT(Id) c FROM Account GROUP BY Rating") {
			got[record.Fields["Rating"].String] = true
		}
		if len(got) != 2 || !got["Hot Label"] || !got["Warm Label"] {
			t.Fatalf("groups = %v", got)
		}
	})
	t.Run("unaliased_ungrouped_error", func(t *testing.T) {
		_, err := ParseAndExecute(org, "SELECT toLabel(Rating), COUNT(Id) c FROM Account GROUP BY Name")
		var queryErr *QueryError
		if !errors.As(err, &queryErr) || queryErr.Uncatchable || queryErr.Message != "Field must be grouped or aggregated: Rating" {
			t.Fatalf("err = %#v", err)
		}
	})
	t.Run("non_aggregate_unchanged", func(t *testing.T) {
		records := run(t, "SELECT Name, toLabel(Rating) FROM Account ORDER BY Name")
		var labels []string
		for _, record := range records {
			// The soql layer keys an unaliased row item by its expression.
			labels = append(labels, record.Fields["TOLABEL(Rating)"].String)
		}
		if len(labels) != 3 || labels[0] != "Hot Label" || labels[1] != "Warm Label" || labels[2] != "Hot Label" {
			t.Fatalf("labels = %v", labels)
		}
	})
}
