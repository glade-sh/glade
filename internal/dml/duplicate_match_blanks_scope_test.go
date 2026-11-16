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

// A record inserted as an allowed duplicate is compared with its insert-time
// values on a later update in the same transaction (native RPT rows). The rows
// cover only that shape: a record inserted without a match keeps the stored-value
// comparison, so moving it away and back after another record took its original
// value is blocked (documented gap in docs/KNOWN_GAPS.md).
func TestUpdateGateBaselineCoversOnlyAllowedDuplicateInserts(t *testing.T) {
	for _, tc := range []struct {
		name        string
		firstEmail  string
		wantBlocked bool
	}{
		{"insertedAsDuplicate", "a@example.invalid", false},
		{"insertedWithoutMatch", "b@example.invalid", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
			storage.EnsureStandardObject(&org, "Contact")
			match := storage.MatchingRule{Object: "Contact", Name: "Blank_Exact", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "Email", Method: "Exact", BlankBehavior: "MatchBlanks"}}}
			object := org.Objects["Contact"]
			object.Definition.DuplicateRules = []storage.DuplicateRule{{Name: "BlankRule", Object: "Contact", Active: true, ActionOnInsert: "Allow", ActionOnUpdate: "Block", MatchRules: []storage.MatchingRule{match}}}
			org.Objects["Contact"] = object
			engine := dml.NewEngine(&org)
			engine.Options.EnforceDuplicateRules = true
			contact := func(name, email string) storage.Record {
				return storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue(name), "Email": storage.StringValue(email)}}
			}
			insert := func(record storage.Record) storage.ID {
				t.Helper()
				got := engine.Insert([]storage.Record{record})
				if len(got) != 1 || !got[0].Success {
					t.Fatalf("insert: %+v", got)
				}
				return got[0].ID
			}
			update := func(id storage.ID, email string) dml.Result {
				t.Helper()
				got := engine.Update([]storage.Record{{Object: "Contact", ID: id, Fields: map[string]storage.Value{"Email": storage.StringValue(email)}}})
				if len(got) != 1 {
					t.Fatalf("update: %+v", got)
				}
				return got[0]
			}
			insert(contact("A", "a@example.invalid"))
			target := insert(contact("B", tc.firstEmail))
			if got := update(target, "away@example.invalid"); !got.Success {
				t.Fatalf("move away: %+v", got)
			}
			insert(contact("C", tc.firstEmail))
			got := update(target, tc.firstEmail)
			blocked := !got.Success && got.StatusCode == "DUPLICATES_DETECTED"
			if blocked != tc.wantBlocked {
				t.Fatalf("move back: blocked=%v want %v: %+v", blocked, tc.wantBlocked, got)
			}
		})
	}
}
