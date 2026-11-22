package soql

import (
	"encoding/csv"
	"os"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE child query with sibling relationships and a conjunctive semi-join:
// testdata/child_semijoin_siblings/org.tsv:2-3 anonymous 62.0 and 67.0.
// The same query also passed the isTest route at lines 6-7.
func TestChildSemiJoinSiblingNativeConformance(t *testing.T) {
	f, err := os.Open("testdata/child_semijoin_siblings/org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	r := csv.NewReader(f)
	r.Comma = '\t'
	rows, err := r.ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	seen := make(map[string]bool)
	for _, row := range rows[1:] {
		if row[0] != "query" {
			continue
		}
		if row[2] != "anonymous" || row[3] != "True" || row[4] != "True" || row[5] != "QUERY=accepted" {
			t.Fatalf("unexpected native row: %q", row)
		}
		if seen[row[1]] {
			t.Fatalf("duplicate native version: %q", row[1])
		}
		seen[row[1]] = true
		t.Run(row[1], func(t *testing.T) {
			_, err := Parse("SELECT Id, (SELECT Id FROM Contacts WHERE Email != null), (SELECT Id, Name FROM Opportunities WHERE StageName = 'Prospecting' AND Id IN (SELECT OpportunityId FROM OpportunityContactRole WHERE Role = 'Decision Maker') LIMIT 1), (SELECT Id FROM Cases LIMIT 1) FROM Account")
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	if len(seen) != 2 || !seen["62.0"] || !seen["67.0"] {
		t.Fatalf("missing native versions: %v", seen)
	}
}

func TestChildSemiJoinSiblingUncapturedNeighborRejected(t *testing.T) {
	const query = "SELECT Id, (SELECT Id FROM Contacts WHERE Email != null), (SELECT Id, Name FROM Opportunities WHERE StageName = 'Prospecting' AND Id IN (SELECT OpportunityId FROM OpportunityContactRole WHERE Role = 'Decision Maker') ORDER BY Name LIMIT 1), (SELECT Id FROM Cases LIMIT 1) FROM Account"
	_, err := Parse(query)
	if err == nil || err.Error() != "soql: semi-join and anti-join subqueries are allowed only in the main WHERE clause" {
		t.Fatalf("uncaptured ordered child: %v", err)
	}
}

func TestChildSemiJoinSiblingUncapturedModifiersRejected(t *testing.T) {
	const base = "SELECT Id, (SELECT Id FROM Contacts WHERE Email != null), (SELECT Id, Name FROM Opportunities WHERE StageName = 'Prospecting' AND Id IN (SELECT OpportunityId FROM OpportunityContactRole WHERE Role = 'Decision Maker') LIMIT 1), (SELECT Id FROM Cases LIMIT 1) FROM Account"
	for _, tc := range []struct{ name, query string }{
		{"parent_limit", base + " LIMIT 1"},
		{"anti_join", strings.Replace(base, "Id IN (SELECT OpportunityId", "Id NOT IN (SELECT OpportunityId", 1)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			_, err := Parse(tc.query)
			if err == nil || err.Error() != "soql: semi-join and anti-join subqueries are allowed only in the main WHERE clause" {
				t.Fatalf("uncaptured modifier: %v", err)
			}
		})
	}
}

func TestChildSemiJoinSiblingConstructedModifierRejections(t *testing.T) {
	const source = "SELECT Id, (SELECT Id FROM Contacts WHERE Email != null), (SELECT Id, Name FROM Opportunities WHERE StageName = 'Prospecting' AND Id IN (SELECT OpportunityId FROM OpportunityContactRole WHERE Role = 'Decision Maker') LIMIT 1), (SELECT Id FROM Cases LIMIT 1) FROM Account"
	for _, tc := range []struct {
		name   string
		change func(*Query)
	}{
		{"parent_scope", func(q *Query) { q.UsingScope = "Mine" }},
		{"child_update", func(q *Query) { q.ChildQueries[1].Query.ForUpdate = true }},
		{"child_security", func(q *Query) { q.ChildQueries[1].Query.SecurityMode = "USER_MODE" }},
	} {
		t.Run(tc.name, func(t *testing.T) {
			query, err := Parse(source)
			if err != nil {
				t.Fatal(err)
			}
			tc.change(&query)
			const want = "soql: semi-join and anti-join subqueries are allowed only in the main WHERE clause"
			if err := validateSemiAntiQuery(query); err == nil || err.Error() != want {
				t.Fatalf("validation = %v", err)
			}
			if _, err := Execute(storage.NewOrgState(), query); err == nil || err.Error() != want {
				t.Fatalf("execution = %v", err)
			}
		})
	}
}
