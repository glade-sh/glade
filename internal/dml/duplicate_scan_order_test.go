package dml

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// A value index is disabled once its values need more than
// maxDuplicateTermMemoValues groups. The distinct count taken before the
// build must reach the same decision at the bound, without building the
// entry and group maps of an index it disables.
func TestDuplicateValueIndexDisablesAtTheSameBound(t *testing.T) {
	for _, tc := range []struct {
		name     string
		distinct int
		badKey   bool
		disabled bool
	}{
		{"at bound", maxDuplicateTermMemoValues, false, false},
		{"over bound", maxDuplicateTermMemoValues + 1, false, true},
		{"mismatched key", 4, true, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
			storage.EnsureStandardObject(&org, "Lead")
			object := org.Objects["Lead"]
			for i := 0; i < 300; i++ {
				id := storage.ID(fmt.Sprintf("00Q%012d", i+1))
				record := storage.Record{ID: id, Object: "Lead", Fields: map[string]storage.Value{"City": storage.StringValue(fmt.Sprintf("City%d", i%tc.distinct))}}
				if tc.badKey && i == 7 {
					record.ID = ""
				}
				object.PutRecord(id, record)
			}
			org.Objects["Lead"] = object
			rule := storage.DuplicateRule{Object: "Lead", Name: "City", MatchRules: []storage.MatchingRule{{Object: "Lead", Name: "City", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "City", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}}}
			candidate := storage.Record{Object: "Lead", Fields: map[string]storage.Value{"City": storage.StringValue("City1")}}
			want, wantOK := FindDuplicateMatches(&org, rule, candidate, nil)
			if !wantOK || !want.HasRecords() {
				t.Fatal("fixture has no matching records")
			}
			var scan DuplicateMatchScan
			for _, pass := range []string{"first", "second", "third"} {
				got, gotOK := scan.Find(&org, rule, candidate, nil, nil, nil)
				if gotOK != wantOK || !reflect.DeepEqual(got, want) {
					t.Fatalf("%s: got %+v, %t; want %+v, %t", pass, got, gotOK, want, wantOK)
				}
			}
			index := scan.index.fields[duplicateIndexFieldKey{object: "Lead", field: "City"}]
			if index == nil || index.disabled != tc.disabled {
				t.Fatalf("index state: %+v, want disabled=%t", index, tc.disabled)
			}
			if index.disabled && (index.entries != nil || index.groups != nil || index.unknown != nil) {
				t.Fatalf("a disabled index kept its maps: %+v", index)
			}
		})
	}
}

// An Exact-only rule tests exclude only for records that match, so a scan
// over mostly different values does not look up every stored ID. Excluded
// matches are still left out, exactly as if they were not stored.
func TestDuplicateExactRuleExcludesOnlyMatches(t *testing.T) {
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Contact")
	object := org.Objects["Contact"]
	excluded := map[storage.ID]bool{}
	for i := 0; i < 400; i++ {
		id := storage.ID(fmt.Sprintf("003%012d", i+1))
		email := fmt.Sprintf("other%d@example.invalid", i)
		if i%50 == 0 {
			email = "Same@Example.invalid"
			if i%100 == 0 {
				excluded[id] = true
			}
		}
		object.PutRecord(id, storage.Record{ID: id, Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("X"), "Email": storage.StringValue(email)}})
	}
	org.Objects["Contact"] = object
	matches := 400 / 50
	rule := storage.DuplicateRule{Object: "Contact", Name: "Email", MatchRules: []storage.MatchingRule{{Object: "Contact", Name: "Email", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "Email", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}}}
	candidate := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"Email": storage.StringValue("same@example.invalid")}}

	trimmed := storage.SnapshotRuntimeOrg(&org)
	storage.EnsureMutableObjectRecords(&trimmed, "Contact")
	for id := range excluded {
		delete(trimmed.Objects["Contact"].Records, id)
	}
	want, wantOK := FindDuplicateMatches(&trimmed, rule, candidate, nil)
	if !wantOK || len(want.Matches) != 1 || len(want.Matches[0].Records) != matches-len(excluded) {
		t.Fatalf("reference: %+v, %t", want, wantOK)
	}
	calls := 0
	exclude := func(id storage.ID) bool {
		calls++
		return excluded[id]
	}
	got, gotOK := FindDuplicateMatches(&org, rule, candidate, exclude)
	if gotOK != wantOK || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %t; want %+v, %t", got, gotOK, want, wantOK)
	}
	if calls != matches {
		t.Fatalf("exclude ran %d times; want once per match (%d)", calls, matches)
	}
}
