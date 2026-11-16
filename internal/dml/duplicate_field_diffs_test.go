package dml_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

func fieldDiffOrg(t *testing.T) (*storage.OrgState, dml.Engine, storage.DuplicateRule) {
	t.Helper()
	org := &storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(org, "Contact")
	engine := dml.NewEngine(org)
	seed := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Diff"), "Email": storage.StringValue("same@example.invalid")}}
	if got := engine.Insert([]storage.Record{seed}); len(got) != 1 || !got[0].Success {
		t.Fatalf("seed: %+v", got)
	}
	match := storage.MatchingRule{Object: "Contact", Name: "Diff_Exact", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "Email", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}
	rule := storage.DuplicateRule{Name: "DiffRule", Object: "Contact", Active: true, ActionOnInsert: "Block", MatchRules: []storage.MatchingRule{match}}
	object := org.Objects["Contact"]
	object.Definition.DuplicateRules = []storage.DuplicateRule{rule}
	org.Objects["Contact"] = object
	return org, engine, rule
}

// NATIVE FieldDiffs of custom Exact rules ../apextest/testdata/conformance/duplicate_runtime_behavior/org.tsv FDIFF anonymous 62.0
// The matcher stays free of field comparisons; a caller that reports the result
// (Find, or the error of a blocked save) attaches them to the matches it shows.
func TestDuplicateFieldDiffsAreAttachedNotMatched(t *testing.T) {
	org, _, rule := fieldDiffOrg(t)
	candidate := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Other"), "Email": storage.StringValue("same@example.invalid")}}
	found, ok := dml.FindDuplicateMatches(org, rule, candidate, nil)
	if !ok || !found.HasRecords() || len(found.Matches) != 1 {
		t.Fatalf("find: %+v %v", found, ok)
	}
	if found.Matches[0].FieldDiffs != nil {
		t.Fatalf("the matcher computed field differences: %+v", found.Matches[0].FieldDiffs)
	}
	found.AttachFieldDiffs(rule, candidate)
	diffs := found.Matches[0].FieldDiffs
	if len(diffs) != 1 || len(diffs[0]) != 1 || diffs[0][0].Name != "Email" || diffs[0][0].Difference != "Same" {
		t.Fatalf("attached differences: %+v", diffs)
	}
}

func TestBlockedDuplicateSaveReportsFieldDiffs(t *testing.T) {
	_, engine, _ := fieldDiffOrg(t)
	row := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Other"), "Email": storage.StringValue("same@example.invalid")}}
	engine.Options.EnforceDuplicateRules = true
	got := engine.Insert([]storage.Record{row})
	if len(got) != 1 || got[0].Success || len(got[0].Errors) != 1 || got[0].Errors[0].Duplicate == nil {
		t.Fatalf("insert: %+v", got)
	}
	matches := got[0].Errors[0].Duplicate.Matches
	if len(matches) != 1 || len(matches[0].FieldDiffs) != 1 || len(matches[0].FieldDiffs[0]) != 1 || matches[0].FieldDiffs[0][0].Difference != "Same" {
		t.Fatalf("blocked save lost its field differences: %+v", matches)
	}
}
