package dml

import (
	"fmt"
	"reflect"
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

func TestDuplicateIndexedAccessAndMutation(t *testing.T) {
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Lead")
	object := org.Objects["Lead"]
	for i := 0; i < 400; i++ {
		id := storage.ID(fmt.Sprintf("00Q%012d", i+1))
		object.PutRecord(id, storage.Record{ID: id, Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue(fmt.Sprintf("%d", i))}})
	}
	org.Objects["Lead"] = object
	rule := storage.DuplicateRule{Object: "Lead", Name: "Name", MatchRules: []storage.MatchingRule{{Object: "Lead", Name: "Name", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "LastName", Method: "LastName", BlankBehavior: "NullNotAllowed"}}}}}
	candidate := storage.Record{Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue("44")}}
	var scan DuplicateMatchScan
	check := func(name string, detect, disclose bool, exclude func(storage.ID) bool) {
		t.Helper()
		access := func(storage.Record) (bool, bool) { return detect, disclose }
		want, wantOK := FindDuplicateMatchesWithAccess(&org, rule, candidate, exclude, access)
		got, gotOK := scan.Find(&org, rule, candidate, exclude, access, nil)
		if gotOK != wantOK || !reflect.DeepEqual(got, want) {
			t.Fatalf("%s: got %+v, %t; want %+v, %t", name, got, gotOK, want, wantOK)
		}
	}
	check("initial", true, true, nil)
	check("indexed", true, true, nil)
	if len(scan.index.fields) == 0 {
		t.Fatal("second scan did not build the candidate index")
	}
	check("hidden", false, false, nil)
	check("detected without disclosure", true, false, nil)
	check("excluded", true, true, func(id storage.ID) bool { return id == "00Q000000000045" })
	object = org.Objects["Lead"]
	target := object.Records["00Q000000000001"]
	target.Fields["LastName"] = storage.StringValue("44")
	check("direct field mutation", true, true, nil)
	target.Fields["LastName"] = storage.StringValue("other")
	object.DeleteRecord("00Q000000000045")
	org.Objects["Lead"] = object
	check("delete and mutation", true, true, nil)
	numeric := object.Records["00Q000000000003"]
	numeric.Fields["LastName"] = storage.IntegerValue(44)
	check("non-string value", true, true, nil)
	candidate.Fields["LastName"] = storage.StringValue("other")
	check("candidate changed", true, true, nil)
	target = target.Clone()
	target.System.IsDeleted = true
	object.PutRecord(target.ID, target)
	org.Objects["Lead"] = object
	check("soft deleted", true, true, nil)
	records := make(map[storage.ID]storage.Record, len(object.Records))
	for id, record := range object.Records {
		records[id] = record
	}
	replacement := records["00Q000000000002"].Clone()
	replacement.Fields["LastName"] = storage.StringValue("other")
	records[replacement.ID] = replacement
	object.SetRecords(records)
	org.Objects["Lead"] = object
	check("record map replaced", true, true, nil)
	rule.MatchRules[0].Items[0].Method = "Exact"
	check("rule method changed", true, true, nil)
	rule.MatchRules[0].Items[0].Field = "FirstName"
	candidate.Fields["FirstName"] = storage.StringValue("other")
	check("rule field changed", true, true, nil)
}

func TestDuplicateIndexedReentrantAccess(t *testing.T) {
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Lead")
	object := org.Objects["Lead"]
	for i := 0; i < 400; i++ {
		id := storage.ID(fmt.Sprintf("00Q%012d", i+1))
		name := fmt.Sprintf("%d", i)
		if i < 4 {
			name = "Smith"
		} else if i < 8 {
			name = "Jones"
		}
		object.PutRecord(id, storage.Record{ID: id, Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue(name)}})
	}
	org.Objects["Lead"] = object
	rule := storage.DuplicateRule{Object: "Lead", Name: "Name", MatchRules: []storage.MatchingRule{{Object: "Lead", Name: "Name", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "LastName", Method: "LastName", BlankBehavior: "NullNotAllowed"}}}}}
	outer := storage.Record{Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue("Smith")}}
	inner := storage.Record{Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue("Jones")}}
	want, ok := FindDuplicateMatches(&org, rule, outer, nil)
	if !ok || len(want.Matches[0].Records) != 4 {
		t.Fatalf("reference: %+v", want)
	}
	var scan DuplicateMatchScan
	if _, ok := scan.Find(&org, rule, outer, nil, nil, nil); !ok {
		t.Fatal("first scan unsupported")
	}
	called := false
	got, ok := scan.Find(&org, rule, outer, nil, func(storage.Record) (bool, bool) {
		if !called {
			called = true
			result, supported := scan.Find(&org, rule, inner, nil, nil, nil)
			if !supported || len(result.Matches[0].Records) != 4 {
				t.Fatalf("nested scan: %+v", result)
			}
		}
		return true, true
	}, nil)
	if !ok || !called || !reflect.DeepEqual(got, want) {
		t.Fatalf("outer scan: %+v; want %+v", got, want)
	}
}

func TestDuplicateIndexedRequiredValues(t *testing.T) {
	for _, tc := range []struct {
		name, method string
		distinct     bool
	}{
		{"exact", "Exact", false},
		{"first name", "FirstName", false},
		{"company", "CompanyName", false},
		{"phone", "Phone", false},
		{"city", "City", false},
		{"street", "Street", false},
		{"zip", "Zip", false},
		{"title", "Title", false},
		{"distinct fallback", "Exact", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			positive, negative := "Alpha", "Beta"
			switch tc.method {
			case "Phone":
				positive, negative = "4155550199", "2125550177"
			case "Zip":
				positive, negative = "98101", "10001"
			case "Street":
				positive, negative = "12 Main St", "99 Oak Rd"
			case "Title":
				positive, negative = "Director Finance", "Research Assistant"
			}
			org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
			storage.EnsureStandardObject(&org, "Lead")
			object := org.Objects["Lead"]
			for i := 0; i < 400; i++ {
				id := storage.ID(fmt.Sprintf("00Q%012d", i+1))
				value := negative
				if i%100 == 0 {
					value = positive
				}
				if tc.distinct {
					value = fmt.Sprintf("Value%d", i)
				}
				fields := map[string]storage.Value{"City": storage.StringValue(value)}
				if i == 1 {
					fields = map[string]storage.Value{"city": storage.StringValue(positive)}
				} else if i == 2 {
					fields["City"] = storage.NullValue()
				}
				object.PutRecord(id, storage.Record{ID: id, Object: "Lead", Fields: fields})
			}
			org.Objects["Lead"] = object
			rule := storage.DuplicateRule{Object: "Lead", Name: "City", MatchRules: []storage.MatchingRule{{Object: "Lead", Name: "City", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "City", Method: tc.method, BlankBehavior: "NullNotAllowed"}}}}}
			candidate := storage.Record{Object: "Lead", Fields: map[string]storage.Value{"City": storage.StringValue(positive)}}
			var scan DuplicateMatchScan
			check := func(name string) {
				t.Helper()
				want, wantOK := FindDuplicateMatches(&org, rule, candidate, nil)
				got, gotOK := scan.Find(&org, rule, candidate, nil, nil, nil)
				if name == "first" && (!wantOK || !want.HasRecords()) {
					t.Fatal("fixture has no matching records")
				}
				if gotOK != wantOK || !reflect.DeepEqual(got, want) {
					t.Fatalf("%s: got %+v, %t; want %+v, %t", name, got, gotOK, want, wantOK)
				}
			}
			check("first")
			check("second")
			index := scan.index.fields[duplicateIndexFieldKey{object: "Lead", field: "City"}]
			if index == nil || index.disabled != tc.distinct {
				t.Fatalf("index state: %+v", index)
			}
			row := object.Records["00Q000000000004"]
			row.Fields["City"] = storage.StringValue(positive)
			check("direct mutation")
			candidate.Fields["City"] = storage.StringValue(negative)
			check("candidate changed")
		})
	}
}

func TestDuplicateIndexedMismatchedRecordIDFallsBack(t *testing.T) {
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Lead")
	object := org.Objects["Lead"]
	for i := 0; i < 300; i++ {
		id := storage.ID(fmt.Sprintf("00Q%012d", i+1))
		record := storage.Record{ID: id, Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue("Smith")}}
		if i == 0 {
			record.ID = ""
		}
		object.PutRecord(id, record)
	}
	org.Objects["Lead"] = object
	rule := storage.DuplicateRule{Object: "Lead", Name: "Name", MatchRules: []storage.MatchingRule{{Object: "Lead", Name: "Name", Status: "Active", Items: []storage.MatchingRuleItem{{Field: "LastName", Method: "LastName", BlankBehavior: "NullNotAllowed"}}}}}
	candidate := storage.Record{Object: "Lead", Fields: map[string]storage.Value{"LastName": storage.StringValue("Smith")}}
	want, wantOK := FindDuplicateMatches(&org, rule, candidate, nil)
	var scan DuplicateMatchScan
	_, _ = scan.Find(&org, rule, candidate, nil, nil, nil)
	got, gotOK := scan.Find(&org, rule, candidate, nil, nil, nil)
	field := scan.index.fields[duplicateIndexFieldKey{object: "Lead", field: "LastName", nameKey: true}]
	if field == nil || !field.disabled || gotOK != wantOK || !reflect.DeepEqual(got, want) {
		t.Fatalf("fallback: index=%+v got=%+v want=%+v", field, got, want)
	}
}
