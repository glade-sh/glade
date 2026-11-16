package dml_test

import (
	"testing"

	"github.com/glade-sh/glade/internal/dml"
	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE Exact MatchBlanks update gate ../apextest/testdata/conformance/duplicate_runtime_behavior/org.tsv GATE_mb and SGATE_mb anonymous 62.0
// The MatchBlanks gate is captured for Apex DML, where EnforceDuplicateRules is
// on. The upsert-by-ID update route without it keeps its shipping behavior: a
// MatchBlanks rule does not run there, so an update into a match saves.
func TestMatchBlanksUpdateGateRequiresDuplicateEnforcement(t *testing.T) {
	for _, enforce := range []bool{false, true} {
		name := "upsertOnly"
		if enforce {
			name = "enforced"
		}
		t.Run(name, func(t *testing.T) {
			org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
			storage.EnsureStandardObject(&org, "Contact")
			engine := dml.NewEngine(&org)
			seed := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Gate"), "Email": storage.StringValue("same@example.invalid")}}
			target := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Gate"), "Email": storage.StringValue("other@example.invalid")}}
			inserted := engine.Insert([]storage.Record{seed, target})
			if len(inserted) != 2 || !inserted[0].Success || !inserted[1].Success {
				t.Fatalf("seed: %+v", inserted)
			}
			match := storage.MatchingRule{Object: "Contact", Name: "Blank_Exact", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "Email", Method: "Exact", BlankBehavior: "MatchBlanks"}}}
			object := org.Objects["Contact"]
			object.Definition.DuplicateRules = []storage.DuplicateRule{{Name: "BlankRule", Object: "Contact", Active: true, ActionOnUpdate: "Block", MatchRules: []storage.MatchingRule{match}}}
			org.Objects["Contact"] = object
			engine.Options.EnforceUpsertIDInsertDuplicateRules = true
			engine.Options.EnforceDuplicateRules = enforce
			row := storage.Record{Object: "Contact", ID: inserted[1].ID, Fields: map[string]storage.Value{"Email": storage.StringValue("same@example.invalid")}}
			got := engine.Upsert([]storage.Record{row})
			if len(got) != 1 {
				t.Fatalf("results: %+v", got)
			}
			blocked := !got[0].Success && got[0].StatusCode == "DUPLICATES_DETECTED"
			if blocked != enforce {
				t.Fatalf("enforce=%v: blocked=%v: %+v", enforce, blocked, got[0])
			}
		})
	}
}
