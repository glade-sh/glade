package soql

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// focused local assertions for accepted aggregate/grouping facets.
func TestExecuteRejectsRollupWithMoreThanThreeFields(t *testing.T) {
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

	queryText := "SELECT COUNT(Id) total FROM Account GROUP BY ROLLUP(Name, Rating, Type, Industry)"
	if _, err := Parse(queryText); err == nil || !strings.Contains(err.Error(), "3 fields or less") {
		t.Fatalf("four-field ROLLUP parse error = %v", err)
	}
	if _, err := ParseAndExecute(org, queryText); err == nil {
		t.Fatal("accepted ROLLUP with more than three grouping fields")
	}
}

func TestExecuteRejectsAverageOnNonNumericField(t *testing.T) {
	_, err := ParseAndExecute(aggregateTestOrg(), "SELECT AVG(Name) averageName FROM Account")
	if err == nil || err.Error() != "soql: AVG requires numeric field Name" {
		t.Fatalf("AVG on text field error = %v", err)
	}
}

func TestExecuteRejectsAggregateLimitWithoutGroupBy(t *testing.T) {
	queryText := "SELECT MAX(AnnualRevenue) maxRevenue FROM Account LIMIT 1"
	if _, err := Parse(queryText); err == nil || !strings.Contains(err.Error(), "cannot also use LIMIT") {
		t.Fatalf("non-grouped aggregate LIMIT parse error = %v", err)
	}
	if _, err := ParseAndExecute(aggregateTestOrg(), queryText); err == nil {
		t.Fatal("accepted aggregate LIMIT without GROUP BY")
	}
}

func TestExecuteRejectsUngroupedSelectedFieldInAggregateQuery(t *testing.T) {
	queryText := "SELECT Name, COUNT(Id) accountCount FROM Account GROUP BY Rating"
	_, err := ParseAndExecute(aggregateTestOrg(), queryText)
	if err == nil || err.Error() != "soql: field Name must be grouped or aggregated" {
		t.Fatalf("selected ungrouped field error = %v", err)
	}
}

func TestExecuteRejectsUngroupedExpandedStandardFields(t *testing.T) {
	org := aggregateTestOrg()
	for _, queryText := range []string{
		"SELECT FIELDS(STANDARD) FROM Account GROUP BY Id",
		"SELECT FIELDS(STANDARD), MIN(AnnualRevenue) FROM Account GROUP BY Id",
		"SELECT FIELDS(STANDARD), MIN(AnnualRevenue) FROM Account",
	} {
		if _, err := ParseAndExecute(org, queryText); err == nil || !strings.Contains(err.Error(), "must be grouped or aggregated") {
			t.Fatalf("ungrouped expanded fields error for %q = %v", queryText, err)
		}
	}
	if _, err := ExecuteWithCache(org, Query{
		Object: "Account", Fields: []string{"FIELDS(STANDARD)"}, GroupBy: []string{"Id"},
	}, nil); err == nil || !strings.Contains(err.Error(), "must be grouped or aggregated") {
		t.Fatalf("constructed query ungrouped expanded fields error = %v", err)
	}
	if _, err := ExecuteWithCache(org, Query{
		Object: "Account", Fields: []string{"FIELDS(STANDARD)", "MIN(AnnualRevenue)"},
	}, nil); err == nil || !strings.Contains(err.Error(), "must be grouped or aggregated") {
		t.Fatalf("constructed global aggregate ungrouped expanded fields error = %v", err)
	}
	for _, queryText := range []string{
		"SELECT FIELDS(STANDARD) FROM Account",
		"SELECT Id, MIN(AnnualRevenue) FROM Account GROUP BY Id",
	} {
		if _, err := ParseAndExecute(org, queryText); err != nil {
			t.Fatalf("valid query %q: %v", queryText, err)
		}
	}
}

func TestAggregateRestrictionBoundariesAndConstructedQueryBypass(t *testing.T) {
	org := aggregateTestOrg()
	for _, queryText := range []string{
		"SELECT COUNT() FROM Account LIMIT 1",
		"SELECT MAX(AnnualRevenue) maxRevenue FROM Account GROUP BY Rating LIMIT 1",
		"SELECT COUNT(Id) total FROM Account GROUP BY ROLLUP(Name, Rating, Id)",
		"SELECT COUNT(Id) total FROM Account GROUP BY CUBE(Name, Rating, Id)",
	} {
		t.Run(queryText, func(t *testing.T) {
			if _, err := ParseAndExecute(org, queryText); err != nil {
				t.Fatal(err)
			}
		})
	}
	for _, queryText := range []string{
		"SELECT MAX(AnnualRevenue) maxRevenue FROM Account LIMIT 0",
		"SELECT MAX(AnnualRevenue) maxRevenue FROM Account LIMIT :rowLimit",
		"SELECT COUNT(Id) total FROM Account LIMIT 1",
	} {
		t.Run(queryText, func(t *testing.T) {
			if _, err := Parse(queryText); err == nil {
				t.Fatal("accepted non-grouped aggregate LIMIT")
			}
		})
	}
	for _, query := range []Query{
		{Object: "Account", Fields: []string{"COUNT(Id)"}, GroupMode: "ROLLUP", GroupBy: []string{"Name", "Rating", "Type", "Industry"}},
		{Object: "Account", Fields: []string{"Name"}, GroupMode: "ROLLUP", GroupBy: []string{"Name", "Rating", "Type", "Industry"}},
		{Object: "Account", Fields: []string{"MAX(AnnualRevenue)"}, HasLimit: true, Limit: 1},
		{Object: "Account", Fields: []string{"MAX(AnnualRevenue)"}, LimitBind: "rowLimit"},
		{Object: "Account", Fields: []string{"COUNT(Id)"}, Count: true, HasLimit: true, Limit: 1},
		{Object: "Account", Fields: []string{"MAX(AnnualRevenue)"}, Aggregates: []Aggregate{{Func: "COUNT"}}, HasLimit: true, Limit: 1},
	} {
		if _, err := ExecuteWithCache(org, query, nil); err == nil {
			t.Fatalf("constructed query bypassed aggregate restriction: %#v", query)
		}
	}
}
