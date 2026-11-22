package dml

import (
	"fmt"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Local performance contract: a required Last Name match key can discard
// different keys without scoring each stored record. The scores and outcomes
// of the remaining candidates still come from the regular matcher.
func TestDuplicateRequiredKeyReducesScoring(t *testing.T) {
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Lead")
	object := org.Objects["Lead"]
	matching := storage.MatchingRule{Object: "Lead", Name: "Name", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "LastName", Method: "LastName", BlankBehavior: "NullNotAllowed"}}}
	object.Definition.DuplicateRules = []storage.DuplicateRule{{Name: "Name", Object: "Lead", Active: true, ActionOnInsert: "Allow", MatchRules: []storage.MatchingRule{matching}}}
	for i := 0; i < 400; i++ {
		id := storage.ID(fmt.Sprintf("00Q%012d", i+1))
		object.PutRecord(id, storage.Record{ID: id, Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue(fmt.Sprintf("%d", i)), "Company": storage.StringValue("Example")}})
	}
	org.Objects["Lead"] = object
	original := fuzzyScorers["lastname"]
	calls := 0
	fuzzyScorers["lastname"] = func(a, b string, blanks bool) (bool, float64) {
		calls++
		return original(a, b, blanks)
	}
	defer func() { fuzzyScorers["lastname"] = original }()
	engine := NewEngine(&org)
	engine.Options.EnforceDuplicateRules = true
	results := engine.Insert([]storage.Record{
		{Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue("99998"), "Company": storage.StringValue("Example")}},
		{Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue("99999"), "Company": storage.StringValue("Example")}},
	})
	for _, result := range results {
		if !result.Success {
			t.Fatalf("insert: %+v", result)
		}
	}
	if calls >= 200 {
		t.Fatalf("scored %d stored values for two distinct missing keys; want fewer than 200", calls)
	}
}
