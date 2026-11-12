package dml

import (
	"math"
	"strings"
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
		// Fourth run. First Name: the match key is two Double Metaphone
		// characters, the candidate's Name Variant entry is directional, and
		// dictionary names are keyed by their alias.
		{"FirstName", "Vorkel", "Volkel", false, 0},
		{"FirstName", "Vorkel", "Vorpel", true, 96},
		{"FirstName", "Vorkelina", "Forkelina", true, 96},
		{"FirstName", "Zathor", "Zator", false, 0},
		{"FirstName", "Daniel", "Diane", true, 95}, // Jaro-Winkler 0.89 with integer-halved transpositions
		{"FirstName", "Bartholomewsonxy", "Bartholoxewsonxy", true, 97},
		{"FirstName", "Kate", "Katte", false, 0},
		{"FirstName", "Katie", "Katte", true, 95},
		{"FirstName", "Rosa", "Rose", true, 100},
		{"FirstName", "Rose", "Rosa", false, 0},
		{"FirstName", "Leo", "Leona", true, 95},
		{"FirstName", "Jose", "Josef", false, 0},
		{"FirstName", "Paul", "Paula", true, 98},
		{"FirstName", "Helen", "Helena", false, 0},
		// Last Name: the key is four Double Metaphone characters; sound-alike
		// spellings score 100; Keyboard Distance costs follow the key offset.
		{"LastName", "Frost", "Frosr", false, 0},
		{"LastName", "Carter", "Carder", true, 99},
		{"LastName", "Meyer", "Maier", true, 100},
		{"LastName", "Brown", "Braun", true, 100},
		{"LastName", "Walker", "Wallker", false, 0},
		{"LastName", "Lach", "Leach", false, 0},
		{"LastName", "Girard", "Girrard", false, 0},
		{"LastName", "Olivier", "Ollivier", false, 0},
		{"LastName", "Cantu", "Cntu", false, 0},
		{"LastName", "Frank", "Franklin", false, 0},
		// Equal Metaphone 3 codes: a dropped vowel, silent letters and
		// name-specific codes (native rows, API 67.0 and 62.0).
		{"LastName", "Smith", "Smth", true, 100},
		{"LastName", "Knight", "Night", true, 100},
		{"LastName", "Hughes", "Huse", true, 100},
		{"LastName", "Stewart", "Stuart", true, 100},
		{"LastName", "Jackson", "Jaxon", true, 100},
		{"LastName", "Filips", "Philipps", true, 100},
		{"LastName", "Lee", "Leigh", true, 100},
		{"LastName", "Olivier", "Ollivier", false, 0},
		{"LastName", "Pelletier", "Peletier", false, 0},
		// A space or hyphen is a difference the codes do not see.
		{"LastName", "Smith Jones", "Smith-Jones", false, 0},
		{"LastName", "De La Cruz", "Delacruz", false, 0},
		{"LastName", "Mc Donald", "McDonald", false, 0},
		{"LastName", "Richardson", "Richardsom", true, 100},
		{"LastName", "Richardson", "Richardsop", true, 98},
		{"LastName", "Martinez", "Martinezk", true, 99},
		{"Phone", "312 555 O199", "312-555-0199", true, 90}, // a letter is a character of the line section
		{"CompanyName", "AT&T", "AT and T", false, 0},
		{"CompanyName", "PG&E", "Pacific Gas & Electric", true, 100},
		{"CompanyName", "ZQW", "Zorblat Quantum Widgets", false, 0}, // acronyms are a dictionary
		{"CompanyName", "Zorblat Quantum Widgets", "Zorblat Quantum Widgets International", true, 93},
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

// A rule with two fuzzy items reports the mean of the two item confidences:
// natively Company 91 with City 94 gives 92 and Company 100 with City 94
// gives 97 (the P4.MULTI.NameCity row).
func TestFuzzyRuleConfidenceAveragesFuzzyItems(t *testing.T) {
	org := storage.OrgState{APIVersion: "67.0", Objects: map[string]storage.ObjectState{}}
	storage.EnsureStandardObject(&org, "Account")
	state := org.Objects["Account"]
	state.Records = map[storage.ID]storage.Record{}
	existing := storage.Record{ID: "001000000000001", Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue("Hewlett Packard"), "BillingCity": storage.StringValue("Chicago")}}
	state.Records[existing.ID] = existing
	org.Objects["Account"] = state
	rule := storage.DuplicateRule{Name: "NameCity", Object: "Account", Active: true, MatchRules: []storage.MatchingRule{{
		Object: "Account", Name: "NameCity", Status: "Active", BooleanFilter: "1 AND 2",
		Items: []storage.MatchingRuleItem{{Field: "Name", Method: "CompanyName", BlankBehavior: "NullNotAllowed"}, {Field: "BillingCity", Method: "City", BlankBehavior: "NullNotAllowed"}},
	}}}
	for _, c := range []struct {
		name, city string
		confidence float64
	}{{"Hewlett Packard Enterprise", "Chicagoo", 92}, {"Hewlett Packard", "Chicagoo", 97}} {
		candidate := storage.Record{Object: "Account", Fields: map[string]storage.Value{"Name": storage.StringValue(c.name), "BillingCity": storage.StringValue(c.city)}}
		result, supported := FindDuplicateMatches(&org, rule, candidate, nil)
		if !supported || len(result.Matches) != 1 || len(result.Matches[0].Confidences) != 1 {
			t.Fatalf("%s/%s: unexpected result: %+v supported=%v", c.name, c.city, result, supported)
		}
		if got := math.Floor(result.Matches[0].Confidences[0] + 0.5); got != c.confidence {
			t.Errorf("%s/%s: confidence %.2f; native reports %.0f", c.name, c.city, result.Matches[0].Confidences[0], c.confidence)
		}
	}
}

// The Last Name scorer takes any text a record can hold without panicking,
// including very long values and non-letter characters.
func TestFuzzyLastNameHandlesOddInput(t *testing.T) {
	long := strings.Repeat("Wolfeschlegelsteinhausen", 400)
	inputs := []string{"", " ", "-", "'", "''", "\u00e9\u00e8\u00ea", "\u4e2d\u6587", "\U0001F600", "a", "Zzzz", "123", "O'", long, long + "x", strings.Repeat("x", 100000), "\x00\xff\xfe"}
	for _, a := range inputs {
		for _, b := range inputs {
			FuzzyScore("LastName", a, b, false)
			FuzzyScore("LastName", a, b, true)
		}
	}
}
