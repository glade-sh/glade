package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Local performance contract: the per-scan term memo only skips records the
// pairwise scorer rejects. Distinct, repeated and mixed stored values, fuzzy
// and Exact required terms, all return the pairwise matches and confidences.
func TestDuplicateTermMemoMatchesPairwiseScan(t *testing.T) {
	firsts := []string{"James", "Mary", "Robert", "Patricia", "John"}
	lasts := []string{"Smith", "Johnson", "Brown", "Garcia", "Miller", "Davis"}
	for _, tc := range []struct {
		name    string
		items   []storage.MatchingRuleItem
		value   func(i int) map[string]storage.Value
		wantOff bool
	}{
		{"distinct fuzzy", []storage.MatchingRuleItem{{Field: "Company", Method: "CompanyName", BlankBehavior: "NullNotAllowed"}}, func(i int) map[string]storage.Value {
			return map[string]storage.Value{"Company": storage.StringValue(fmt.Sprintf("Example %d Works", i%397))}
		}, true},
		{"repeated fuzzy", []storage.MatchingRuleItem{
			{Field: "FirstName", Method: "FirstName", BlankBehavior: "NullNotAllowed"},
			{Field: "LastName", Method: "LastName", BlankBehavior: "NullNotAllowed"},
		}, func(i int) map[string]storage.Value {
			return map[string]storage.Value{"FirstName": storage.StringValue(firsts[i%len(firsts)]), "LastName": storage.StringValue(lasts[(i/len(firsts))%len(lasts)])}
		}, false},
		{"mixed fuzzy", []storage.MatchingRuleItem{{Field: "LastName", Method: "LastName", BlankBehavior: "NullNotAllowed"}}, func(i int) map[string]storage.Value {
			if i%4 != 0 {
				return map[string]storage.Value{"LastName": storage.StringValue(lasts[i%len(lasts)])}
			}
			return map[string]storage.Value{"LastName": storage.StringValue(fmt.Sprintf("Smith%d", i))}
		}, false},
		{"exact", []storage.MatchingRuleItem{{Field: "Email", Method: "Exact", BlankBehavior: "NullNotAllowed"}}, func(i int) map[string]storage.Value {
			return map[string]storage.Value{"Email": storage.StringValue(fmt.Sprintf("p%d@example.invalid", i%50))}
		}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
			storage.EnsureStandardObject(&org, "Lead")
			object := org.Objects["Lead"]
			for i := 0; i < 600; i++ {
				id := storage.ID(fmt.Sprintf("00Q%012d", i+1))
				fields := tc.value(i)
				if fields["Company"].Kind != storage.ValueString {
					fields["Company"] = storage.StringValue("Example")
				}
				object.PutRecord(id, storage.Record{ID: id, Object: "Lead", Fields: fields, System: storage.SystemFields{CreatedDate: "2023-01-01T00:00:00Z"}})
			}
			org.Objects["Lead"] = object
			matching := storage.MatchingRule{Object: "Lead", Name: "Memo", Status: "Active", Items: tc.items}
			rule := storage.DuplicateRule{Name: "Memo", Object: "Lead", Active: true, ActionOnInsert: "Allow", MatchRules: []storage.MatchingRule{matching}}
			for _, probe := range []int{3, 41, 250} {
				candidate := storage.Record{Object: "Lead", Fields: tc.value(probe)}
				keys := customRuleKeyModes(matching)
				want := map[storage.ID]float64{}
				for id, existing := range object.Records {
					matched, supported, confidence := customDuplicateMatch(matching, keys, candidate, existing)
					if !supported {
						t.Fatalf("pairwise scan unsupported for %s", id)
					}
					if matched {
						want[id] = confidence
					}
				}
				if len(want) == 0 || len(want) > 100 {
					t.Fatalf("probe %d: fixture has %d pairwise matches; want 1..100", probe, len(want))
				}
				result, supported := FindDuplicateMatches(&org, rule, candidate, nil)
				if !supported || len(result.Matches) != 1 {
					t.Fatalf("probe %d: supported=%t matches=%d", probe, supported, len(result.Matches))
				}
				got := result.Matches[0]
				if len(got.Records) != len(want) {
					t.Fatalf("probe %d: %d matches, pairwise %d", probe, len(got.Records), len(want))
				}
				for i, record := range got.Records {
					if confidence, ok := want[record.ID]; !ok || confidence != got.Confidences[i] {
						t.Fatalf("probe %d: match %s confidence %v differs from pairwise %v (%t)", probe, record.ID, got.Confidences[i], confidence, ok)
					}
				}
				prepared, ok := newPreparedCustomMatch(matching, keys, candidate)
				if !ok {
					t.Fatal("rule did not prepare")
				}
				terms, count := requiredDuplicateTerms(&prepared)
				if count == 0 {
					t.Fatal("no required term")
				}
				if prepared.items[terms[0]].exact {
					// Exact terms are never memoized; the pairwise check above covers them.
					continue
				}
				memo := newDuplicateTermVerdicts(terms[0])
				for id, existing := range object.Records {
					if !memo.mayMatch(&prepared, existing) {
						if _, matched := want[id]; matched {
							t.Fatalf("probe %d: memo rejected pairwise match %s", probe, id)
						}
					}
				}
				if memo.off != tc.wantOff {
					t.Fatalf("probe %d: memo off=%t, want %t", probe, memo.off, tc.wantOff)
				}
				memo.release()
			}
		})
	}
}
