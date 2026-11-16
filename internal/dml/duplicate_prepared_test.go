package dml

import (
	"fmt"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// TestPreparedCustomMatchEqualsPairwise compares the prepared matcher with
// customDuplicateMatch for every pair of a value grid under rules that mix
// Exact and fuzzy items, blank handling, initial-keyed first names and filters
// that are AND, OR, nested, unparsable or out of range. The scans run in
// parallel so the shared plan cache is exercised under -race.
func TestPreparedCustomMatchEqualsPairwise(t *testing.T) {
	long := strings.Repeat("x", 99)
	values := map[string][]storage.Value{
		"Email":     {storage.StringValue("a@example.invalid"), storage.StringValue(" A@Example.invalid "), storage.StringValue("b@example.invalid"), storage.StringValue(""), storage.StringValue("   "), storage.NullValue(), storage.StringValue(long + "1"), storage.StringValue(long + "2")},
		"FirstName": {storage.StringValue("Kristen"), storage.StringValue("Cristen"), storage.StringValue("K"), storage.StringValue("Bill"), storage.StringValue("William"), storage.StringValue(""), storage.NullValue()},
		"LastName":  {storage.StringValue("Volkel"), storage.StringValue("Vorkel"), storage.StringValue("Smith"), storage.StringValue("McAllen"), storage.StringValue("Mc Allen"), storage.StringValue("")},
		"Company":   {storage.StringValue("Acme Inc"), storage.StringValue("ACME Incorporated"), storage.StringValue("Globex LLC"), storage.StringValue("The Acme Co"), storage.StringValue("")},
		"Street":    {storage.StringValue("100 Main St"), storage.StringValue("100 Main Street"), storage.StringValue("12 Oak Ave #5"), storage.StringValue("")},
		"Phone":     {storage.StringValue("(415) 555-0100"), storage.StringValue("415-555-0100"), storage.StringValue("212 555 0199"), storage.StringValue("")},
		"Count":     {storage.IntegerValue(7), storage.StringValue("7"), storage.BooleanValue(true)},
	}
	item := func(field, method, blanks string) storage.MatchingRuleItem {
		return storage.MatchingRuleItem{Field: field, Method: method, BlankBehavior: blanks}
	}
	rules := []storage.MatchingRule{
		{Items: []storage.MatchingRuleItem{item("Email", "Exact", "NullNotAllowed")}},
		{Items: []storage.MatchingRuleItem{item("Email", "Exact", "MatchBlanks")}},
		{Items: []storage.MatchingRuleItem{item("Count", "Exact", "NullNotAllowed")}},
		{Items: []storage.MatchingRuleItem{item("FirstName", "FirstName", "NullNotAllowed"), item("LastName", "LastName", "NullNotAllowed")}},
		{BooleanFilter: "1 OR 2", Items: []storage.MatchingRuleItem{item("FirstName", "FirstName", "NullNotAllowed"), item("LastName", "LastName", "MatchBlanks")}},
		{BooleanFilter: "(1 AND 2) OR 3", Items: []storage.MatchingRuleItem{item("FirstName", "FirstName", "NullNotAllowed"), item("LastName", "LastName", "NullNotAllowed"), item("Email", "Exact", "NullNotAllowed")}},
		{BooleanFilter: "(1 AND 2) OR (3 AND 4)", Items: []storage.MatchingRuleItem{item("FirstName", "FirstName", "NullNotAllowed"), item("LastName", "LastName", "NullNotAllowed"), item("Company", "CompanyName", "NullNotAllowed"), item("Street", "Street", "MatchBlanks")}},
		{BooleanFilter: "1 AND (2 OR 3)", Items: []storage.MatchingRuleItem{item("Phone", "Phone", "NullNotAllowed"), item("Company", "CompanyName", "NullNotAllowed"), item("Email", "Exact", "MatchBlanks")}},
		{BooleanFilter: "1 AND", Items: []storage.MatchingRuleItem{item("Email", "Exact", "NullNotAllowed"), item("FirstName", "FirstName", "NullNotAllowed")}},
		{BooleanFilter: "1 OR 3", Items: []storage.MatchingRuleItem{item("Email", "Exact", "NullNotAllowed"), item("LastName", "LastName", "NullNotAllowed")}},
		{BooleanFilter: "NOT(1)", Items: []storage.MatchingRuleItem{item("Email", "Exact", "NullNotAllowed")}},
	}
	records := func(fields []string) []storage.Record {
		out := []storage.Record{{Object: "Lead", Fields: map[string]storage.Value{}}}
		for _, field := range fields {
			var next []storage.Record
			for _, record := range out {
				for _, value := range values[field] {
					clone := record.Clone()
					clone.Fields[field] = value
					next = append(next, clone)
				}
				next = append(next, record) // field absent
			}
			out = next
		}
		return out
	}
	var wg sync.WaitGroup
	for r, rule := range rules {
		wg.Add(1)
		go func() {
			defer wg.Done()
			var fields []string
			for _, item := range rule.Items {
				fields = append(fields, item.Field)
			}
			grid := records(fields)
			if len(grid) > 250 {
				// Keep an even sample so every field still varies.
				sample := make([]storage.Record, 0, 250)
				for i := 0; i < len(grid); i += (len(grid) + 249) / 250 {
					sample = append(sample, grid[i])
				}
				grid = sample
			}
			keys := customRuleKeyModes(rule)
			for _, candidate := range grid {
				prepared, ok := newPreparedCustomMatch(rule, keys, candidate)
				if !ok {
					t.Errorf("rule %d: not prepared", r)
					return
				}
				for _, existing := range grid {
					gm, gs, gc := prepared.match(existing)
					wm, ws, wc := customDuplicateMatch(rule, keys, candidate, existing)
					if gm != wm || gs != ws || gc != wc {
						t.Errorf("rule %d %q: candidate %s existing %s: got (%v %v %v), want (%v %v %v)", r, rule.BooleanFilter, fmt.Sprint(candidate.Fields), fmt.Sprint(existing.Fields), gm, gs, gc, wm, ws, wc)
						return
					}
				}
			}
		}()
	}
	wg.Wait()
}
