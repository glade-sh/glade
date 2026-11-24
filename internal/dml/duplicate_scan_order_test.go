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
		{"far over bound", 300, false, true},
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
			matching := storage.MatchingRule{Object: "Lead", Name: "City", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "City", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}
			rule := storage.DuplicateRule{Object: "Lead", Name: "City", MatchRules: []storage.MatchingRule{matching}}
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
			if !tc.disabled || tc.badKey {
				return
			}
			// A first build that ends disabled must not allocate a group map per
			// distinct value before it gives up: only the index's own map, the
			// disabled marker and the bounded distinct count.
			prepared, ok := newPreparedCustomMatch(matching, customRuleKeyModes(matching), candidate)
			if !ok {
				t.Fatal("rule did not prepare")
			}
			records := org.Objects["Lead"].Records
			allocations := testing.AllocsPerRun(20, func() {
				fresh := &duplicateCandidateIndex{}
				if selected := fresh.readIndexedDuplicateCandidates("Lead", &prepared, 0, false, records); selected != nil {
					t.Fatalf("disabled index selected %d candidates", len(selected))
				}
			})
			if allocations > 12 {
				t.Fatalf("a first build that disables the index allocated %.0f times; want at most 12 (no per-value groups)", allocations)
			}
		})
	}
}

// excludeDifferential compares a scan under exclude with the same scan over
// an org without the excluded records, and returns how often exclude ran.
func excludeDifferential(t *testing.T, org storage.OrgState, objectName string, rule storage.DuplicateRule, candidate storage.Record, excluded map[storage.ID]bool, scan func(*storage.OrgState, func(storage.ID) bool) (DuplicateResult, bool)) (DuplicateResult, bool, int) {
	t.Helper()
	trimmed := storage.SnapshotRuntimeOrg(&org)
	storage.EnsureMutableObjectRecords(&trimmed, objectName)
	for id := range excluded {
		delete(trimmed.Objects[objectName].Records, id)
	}
	want, wantOK := FindDuplicateMatches(&trimmed, rule, candidate, nil)
	calls := 0
	exclude := func(id storage.ID) bool {
		calls++
		return excluded[id]
	}
	got, gotOK := scan(&org, exclude)
	if gotOK != wantOK || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %+v, %t; want %+v, %t", got, gotOK, want, wantOK)
	}
	return got, gotOK, calls
}

func plainScan(rule storage.DuplicateRule, candidate storage.Record) func(*storage.OrgState, func(storage.ID) bool) (DuplicateResult, bool) {
	return func(org *storage.OrgState, exclude func(storage.ID) bool) (DuplicateResult, bool) {
		return FindDuplicateMatches(org, rule, candidate, exclude)
	}
}

// emailContactOrg stores contacts where every `every`-th one shares the
// candidate's name and email; every `excludeEvery`-th of those is excluded.
func emailContactOrg(stored, every, excludeEvery int) (storage.OrgState, map[storage.ID]bool, int) {
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Contact")
	object := org.Objects["Contact"]
	excluded := map[storage.ID]bool{}
	matches := 0
	for i := 0; i < stored; i++ {
		id := storage.ID(fmt.Sprintf("003%012d", i+1))
		email := fmt.Sprintf("other%d@example.invalid", i)
		first, last := fmt.Sprintf("Other%d", i), fmt.Sprintf("Person%d", i)
		if i%every == 0 {
			email, first, last = "Same@Example.invalid", "Pat", "Example"
			matches++
			if excludeEvery > 0 && i%excludeEvery == 0 {
				excluded[id] = true
			}
		}
		object.PutRecord(id, storage.Record{ID: id, Object: "Contact", Fields: map[string]storage.Value{"FirstName": storage.StringValue(first), "LastName": storage.StringValue(last), "Email": storage.StringValue(email)}})
	}
	org.Objects["Contact"] = object
	return org, excluded, matches
}

// An Exact-only rule tests exclude only for records that match, so a scan
// over mostly different values does not look up every stored ID. Excluded
// matches are still left out, exactly as if they were not stored.
func TestDuplicateExactRuleExcludesOnlyMatches(t *testing.T) {
	org, excluded, matches := emailContactOrg(400, 50, 100)
	rule := storage.DuplicateRule{Object: "Contact", Name: "Email", MatchRules: []storage.MatchingRule{{Object: "Contact", Name: "Email", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "Email", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}}}
	candidate := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"Email": storage.StringValue("same@example.invalid")}}
	got, _, calls := excludeDifferential(t, org, "Contact", rule, candidate, excluded, plainScan(rule, candidate))
	if len(got.Matches) != 1 || len(got.Matches[0].Records) != matches-len(excluded) {
		t.Fatalf("matches: %+v", got)
	}
	if calls != matches {
		t.Fatalf("exclude ran %d times; want once per match (%d)", calls, matches)
	}
}

// Fuzzy and standard rules keep the old order: their scorers cost more than
// the exclude lookup, so exclude still runs for every stored record first.
func TestDuplicateFuzzyAndStandardRulesExcludeEveryRecord(t *testing.T) {
	const stored = 300
	org, excluded, _ := emailContactOrg(stored, 30, 60)
	fuzzy := storage.DuplicateRule{Object: "Contact", Name: "Name", MatchRules: []storage.MatchingRule{{Object: "Contact", Name: "Name", Status: "Active", Items: []storage.MatchingRuleItem{
		{Field: "LastName", Method: "LastName", BlankBehavior: "NullNotAllowed"},
		{Field: "Email", Method: "Exact", BlankBehavior: "NullNotAllowed"},
	}}}}
	standard := storage.DuplicateRule{Object: "Contact", Name: "Standard", MatchRules: []storage.MatchingRule{{Object: "Contact", Name: "Standard_Contact_Match_Rule_v1_1", Status: "Active"}}}
	if !storage.IsStandardMatchingRule("Contact", standard.MatchRules[0].Name) {
		t.Fatal("fixture rule is not a standard matching rule")
	}
	candidate := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"FirstName": storage.StringValue("Pat"), "LastName": storage.StringValue("Example"), "Email": storage.StringValue("same@example.invalid")}}
	for _, tc := range []struct {
		name string
		rule storage.DuplicateRule
	}{{"fuzzy", fuzzy}, {"standard", standard}} {
		t.Run(tc.name, func(t *testing.T) {
			got, ok, calls := excludeDifferential(t, org, "Contact", tc.rule, candidate, excluded, plainScan(tc.rule, candidate))
			if !ok || !got.HasRecords() {
				t.Fatalf("fixture has no matches: %+v, %t", got, ok)
			}
			if calls != stored {
				t.Fatalf("exclude ran %d times; want once per stored record (%d)", calls, stored)
			}
		})
	}
}

// An unsupported verdict on an excluded record is skipped, as before, and an
// unsupported verdict on any other record still makes the scan unsupported.
func TestDuplicateExactRuleUnsupportedVerdictHonorsExclude(t *testing.T) {
	org, _, _ := emailContactOrg(40, 10, 0)
	// Item 3 does not exist, so every filter outcome is unsupported.
	rule := storage.DuplicateRule{Object: "Contact", Name: "Bad", MatchRules: []storage.MatchingRule{{Object: "Contact", Name: "Bad", Status: "Active", BooleanFilter: "1 OR 3", Items: []storage.MatchingRuleItem{
		{Field: "Email", Method: "Exact", BlankBehavior: "NullNotAllowed"},
		{Field: "LastName", Method: "Exact", BlankBehavior: "NullNotAllowed"},
	}}}}
	candidate := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"Email": storage.StringValue("same@example.invalid"), "LastName": storage.StringValue("Example")}}
	all := map[storage.ID]bool{}
	for id := range org.Objects["Contact"].Records {
		all[id] = true
	}
	if _, ok, calls := excludeDifferential(t, org, "Contact", rule, candidate, all, plainScan(rule, candidate)); !ok || calls != len(all) {
		t.Fatalf("every record excluded: supported=%t calls=%d; want supported, %d calls", ok, calls, len(all))
	}
	some := map[storage.ID]bool{}
	for id := range all {
		if len(some) < len(all)-1 {
			some[id] = true
		}
	}
	if _, ok, _ := excludeDifferential(t, org, "Contact", rule, candidate, some, plainScan(rule, candidate)); ok {
		t.Fatal("one record left unexcluded: want the unsupported verdict")
	}
}

// A Person Account rule skips business accounts before scoring; exclude runs
// only for matching person accounts.
func TestDuplicatePersonAccountExactRuleExcludesOnlyMatches(t *testing.T) {
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Account")
	object := org.Objects["Account"]
	excluded := map[storage.ID]bool{}
	personMatches := 0
	for i := 0; i < 300; i++ {
		id := storage.ID(fmt.Sprintf("001%012d", i+1))
		email := fmt.Sprintf("other%d@example.invalid", i)
		if i%20 == 0 {
			email = "same@example.invalid"
		}
		fields := map[string]storage.Value{"Name": storage.StringValue(fmt.Sprintf("Account %d", i)), "PersonEmail": storage.StringValue(email)}
		if i%40 == 0 {
			fields["IsPersonAccount"] = storage.BooleanValue(true)
			fields["PersonContactId"] = storage.IDValue(storage.ID(fmt.Sprintf("003%012d", i+1)))
			personMatches++
			if i%80 == 0 {
				excluded[id] = true
			}
		} else {
			fields["IsPersonAccount"] = storage.BooleanValue(false)
		}
		object.PutRecord(id, storage.Record{ID: id, Object: "Account", Fields: fields})
	}
	org.Objects["Account"] = object
	rule := storage.DuplicateRule{Object: "Account", Subtype: "PersonAccount", Name: "PersonEmail", MatchRules: []storage.MatchingRule{{Object: "PersonAccount", Name: "PersonEmail", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "PersonEmail", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}}}
	candidate := storage.Record{Object: "Account", Fields: map[string]storage.Value{"PersonEmail": storage.StringValue("same@example.invalid"), "IsPersonAccount": storage.BooleanValue(true)}}
	got, ok, calls := excludeDifferential(t, org, "Account", rule, candidate, excluded, plainScan(rule, candidate))
	if !ok || len(got.Matches) != 1 || len(got.Matches[0].Records) != personMatches-len(excluded) {
		t.Fatalf("person matches: %+v, %t; want %d records", got, ok, personMatches-len(excluded))
	}
	if calls != personMatches {
		t.Fatalf("exclude ran %d times; want once per person-account match (%d)", calls, personMatches)
	}
}

// The indexed path (visit) applies the same late exclude as the full scan.
func TestDuplicateIndexedExactRuleExcludesOnlyMatches(t *testing.T) {
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Lead")
	object := org.Objects["Lead"]
	excluded := map[storage.ID]bool{}
	matches := 0
	for i := 0; i < 400; i++ {
		id := storage.ID(fmt.Sprintf("00Q%012d", i+1))
		city := fmt.Sprintf("City%d", i%10)
		if city == "City3" {
			matches++
			if i%20 == 3 {
				excluded[id] = true
			}
		}
		object.PutRecord(id, storage.Record{ID: id, Object: "Lead", Fields: map[string]storage.Value{"City": storage.StringValue(city)}})
	}
	org.Objects["Lead"] = object
	rule := storage.DuplicateRule{Object: "Lead", Name: "City", MatchRules: []storage.MatchingRule{{Object: "Lead", Name: "City", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "City", Method: "Exact", BlankBehavior: "NullNotAllowed"}}}}}
	candidate := storage.Record{Object: "Lead", Fields: map[string]storage.Value{"City": storage.StringValue("city3")}}
	var scan DuplicateMatchScan
	find := func(org *storage.OrgState, exclude func(storage.ID) bool) (DuplicateResult, bool) {
		return scan.Find(org, rule, candidate, exclude, nil, nil)
	}
	_, _, first := excludeDifferential(t, org, "Lead", rule, candidate, excluded, find)
	_, _, indexed := excludeDifferential(t, org, "Lead", rule, candidate, excluded, find)
	index := scan.index.fields[duplicateIndexFieldKey{object: "Lead", field: "City"}]
	if index == nil || index.disabled {
		t.Fatalf("second scan did not use the value index: %+v", index)
	}
	if first != matches || indexed != matches {
		t.Fatalf("exclude ran %d (scan) and %d (indexed) times; want once per match (%d)", first, indexed, matches)
	}
}
