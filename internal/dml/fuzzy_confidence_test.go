package dml

import (
	"math"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Single-item scores reproduce the native match confidences of the fixture
// rows (API 62.0 and 67.0): the Maximum methods report (100 + score) / 2, the
// weighted methods their section score.
func TestFuzzyScoreNativeConfidences(t *testing.T) {
	cases := []struct {
		method, left, right string
		match               bool
		confidence          float64
	}{
		{"City", "Chicago", "Chicagoo", true, 94},            // edit similarity 87.5
		{"City", "Seattle", "Seatle", false, 0},              // key "seattl" differs from "seatle"
		{"City", "San Francisco", "San Francsico", false, 0}, // same key, similarity 84.6 below 85
		{"Street", "15 Elm St", "15 Elm Drive", true, 70},    // sections 82.35, string similarity 58.3
		{"Street", "123 Main St", "123 Main St Suite 5", true, 79},
		{"Street", "123 Main St Suite 5", "123 Main St Suite 6", true, 90},
		{"Street", "100 Main St", "100 E Main St", true, 92},
		{"Street", "100 Main St", "100 N Main St", false, 0},
		{"Street", "42 Wallaby Way", "42 Wallaby Way Unit 2", false, 0},
		{"Street", "42 Wallaby Way", "42 Wallaby Way Ste 2", true, 75},
		{"FirstName", "John", "Johny", true, 98},      // Jaro-Winkler 0.96
		{"FirstName", "Matt", "Mattew", true, 97},     // Jaro-Winkler 0.933
		{"FirstName", "Robert", "Bob", true, 100},     // Name Variant
		{"FirstName", "Michael", "Micheal", false, 0}, // dictionary keeps them apart
		{"LastName", "Smith", "Smyth", true, 100},
		{"LastName", "Garcia", "García", true, 99},  // keyboard: accent costs 0.16 of 6
		{"LastName", "Walker", "Wallker", false, 0}, // LL before a consonant changes the key
		{"LastName", "Martinez", "Martinpz", true, 97},
		{"CompanyName", "Acme", "Acme GmbH", true, 100},
		{"CompanyName", "Hewlett Packard", "Hewlett Packard Enterprise", true, 91},
		{"CompanyName", "Adobe", "Adobe Systems", false, 0}, // key "adobe" differs from "adobesyste"
		{"CompanyName", "IBM", "International Business Machines", true, 100},
		{"Phone", "415-555-1234", "+1 415 555 1234", true, 90},
		{"Zip", "94105", "94105-1234", true, 100},
	}
	for _, c := range cases {
		match, confidence := FuzzyScore(c.method, c.left, c.right, false)
		if match != c.match || (match && math.Floor(confidence+0.5) != c.confidence) {
			t.Errorf("%s %q ~ %q: match=%v confidence=%.2f; native match=%v confidence=%.0f", c.method, c.left, c.right, match, confidence, c.match, c.confidence)
		}
	}
}

// A rule's confidence is the mean of its item scores: the native Title rows
// pair a fuzzy Title item with an exact Last Name item and report 93 for one
// shared word of two.
func TestFuzzyRuleConfidenceAveragesItems(t *testing.T) {
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Contact")
	state := org.Objects["Contact"]
	state.Records = map[storage.ID]storage.Record{}
	existing := storage.Record{ID: "003000000000001", Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Probe"), "Title": storage.StringValue("Sales Manager")}}
	state.Records[existing.ID] = existing
	org.Objects["Contact"] = state
	rule := storage.DuplicateRule{Name: "TitleRule", Object: "Contact", Active: true, MatchRules: []storage.MatchingRule{{
		Object: "Contact", Name: "Title", Status: "Active", BooleanFilter: "1 AND 2",
		Items: []storage.MatchingRuleItem{{Field: "Title", Method: "Title", BlankBehavior: "NullNotAllowed"}, {Field: "LastName", Method: "Exact", BlankBehavior: "NullNotAllowed"}},
	}}}
	candidate := storage.Record{Object: "Contact", Fields: map[string]storage.Value{"LastName": storage.StringValue("Probe"), "Title": storage.StringValue("VP Sales")}}
	result, supported := FindDuplicateMatches(&org, rule, candidate, nil)
	if !supported || len(result.Matches) != 1 || len(result.Matches[0].Records) != 1 || len(result.Matches[0].Confidences) != 1 {
		t.Fatalf("unexpected result: %+v supported=%v", result, supported)
	}
	if got := math.Floor(result.Matches[0].Confidences[0] + 0.5); got != 93 {
		t.Fatalf("confidence %.2f; native reports 93 for VP Sales ~ Sales Manager with an exact last name", result.Matches[0].Confidences[0])
	}
}
