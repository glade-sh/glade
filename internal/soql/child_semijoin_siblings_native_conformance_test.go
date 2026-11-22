package soql

import (
	"encoding/csv"
	"os"
	"testing"
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
	seen := 0
	for _, row := range rows[1:] {
		if row[0] != "query" {
			continue
		}
		if row[2] != "anonymous" || row[3] != "True" || row[4] != "True" || row[5] != "QUERY=accepted" {
			t.Fatalf("unexpected native row: %q", row)
		}
		seen++
		t.Run(row[1], func(t *testing.T) {
			_, err := Parse("SELECT Id, (SELECT Id FROM Contacts WHERE Email != null), (SELECT Id, Name FROM Opportunities WHERE StageName = 'Prospecting' AND Id IN (SELECT OpportunityId FROM OpportunityContactRole WHERE Role = 'Decision Maker') LIMIT 1), (SELECT Id FROM Cases LIMIT 1) FROM Account")
			if err != nil {
				t.Fatal(err)
			}
		})
	}
	if seen != 2 {
		t.Fatalf("got %d native rows, want 2", seen)
	}
}
