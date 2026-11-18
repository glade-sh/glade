package dml

import (
	"math"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE fuzzy_first_initials/org.tsv (held-out panel hf1): a custom
// FirstName rule at API 62.0 and 67.0, anonymous and @IsTest, two repeats.
// Every ordered pair goes through the shared duplicate-rule matcher and must
// agree exactly: "T" finds "TW", "Tw" and "T.W." at 100 and never "Ty".
func TestFuzzyFirstInitialsNativeRows(t *testing.T) {
	cases, observations := fuzzyNameKeysDataFrom(t, "testdata/conformance/fuzzy_first_initials/")
	c, ok := cases["hf1"]
	if !ok || len(cases) != 1 || c.Kind != "panel" {
		t.Fatalf("cases %v", cases)
	}
	native := fuzzyNameKeysPanel(t, observations["hf1"], len(c.Values))
	org, ids := fuzzyNameKeysOrg(c, "Allow", c.Values...)
	index := map[storage.ID]int{}
	for i, id := range ids {
		index[id] = i
	}
	rule := fuzzyNameKeysRule(c, "Allow")
	var counts fuzzyNameKeysCount
	for i, value := range c.Values {
		got := map[int]float64{}
		result, supported := FindDuplicateMatches(org, rule, fuzzyNameKeysRecord(c, value), nil)
		if !supported {
			t.Fatal("rule not supported")
		}
		for _, match := range result.Matches {
			for k, record := range match.Records {
				got[index[record.ID]] = math.Floor(match.Confidences[k] + 0.5)
			}
		}
		for j := range c.Values {
			n, g := native[[2]int{i, j}], got[j]
			switch {
			case n == 0 && g == 0:
				continue
			case n == g:
				counts.native++
				counts.hit++
				continue
			case n > 0 && g > 0:
				counts.native++
				counts.hit++
				counts.confidence++
			case n > 0:
				counts.native++
				counts.miss++
			default:
				counts.extra++
			}
			t.Errorf("hf1/%s=>%s: native=%v glade=%v", value, c.Values[j], n, g)
		}
	}
	t.Logf("hf1: %s", counts)
	if want := (fuzzyNameKeysCount{native: 84, hit: 84}); !t.Failed() && counts != want {
		t.Errorf("hf1 counts %s, want %s", counts, want)
	}
}

// NATIVE fuzzy_first_alias_keys/org.tsv:2-17 (i1a, i1b) and
// fuzzy_first_initials/org.tsv (hf1), custom FirstName rule, API 62.0 and
// 67.0, anonymous and @IsTest: "Ty", "Ray" and "Sue" are keyed by an alias
// ("Tyr", "Raym" and "Sues" share it), so neither an initial nor a neighbor
// under their own spelling finds them. Initials and Jaro-Winkler are
// unchanged for every other name.
func TestFuzzyFirstNameCustomAliasKeys(t *testing.T) {
	for _, c := range []struct {
		a, b       string
		match      bool
		confidence float64
	}{
		// hf1.
		{"T", "Ty", false, 0},
		{"Ty", "T", false, 0},
		{"T.", "Ty", false, 0},
		{"T W", "Ty", false, 0},
		{"T. W.", "Ty", false, 0},
		{"Ty", "T L", false, 0},
		{"T", "TW", true, 100},
		{"T", "Tw", true, 100},
		{"T", "T.W.", true, 100},
		{"Ty", "Ty", true, 100},
		// i1a and i1b: "Ty".
		{"Ty", "Tye", false, 0},
		{"Tye", "Ty", false, 0},
		{"Ty", "Tyh", false, 0},
		{"Tyw", "Ty", false, 0},
		{"Ty", "Tyl", false, 0},
		{"Ty", "Tya", false, 0},
		{"Ty", "Tyrone", false, 0},
		{"Ty", "Tyr", true, 96},
		{"Tyr", "Ty", true, 96},
		{"T", "Tye", true, 100},
		{"Tye", "T", true, 100},
		{"Tye", "Tyeh", true, 97},
		{"T", "Tyr", false, 0},
		// i1a and i1b: "Ray".
		{"R", "Ray", false, 0},
		{"Ray", "R", false, 0},
		{"Ray", "Rae", false, 0},
		{"Rae", "Ray", false, 0},
		{"Ray", "Raye", false, 0},
		{"Ray", "Ra", false, 0},
		{"Ray", "Rayh", false, 0},
		{"Ray", "Raym", true, 97},
		{"Raym", "Ray", true, 97},
		{"Ray", "Raymond", true, 100},
		{"R", "Rae", true, 100},
		{"Rae", "Raye", true, 97},
		// i1a and i1b: "Sue".
		{"S", "Sue", false, 0},
		{"Sue", "S", false, 0},
		{"Sue", "Su", false, 0},
		{"Su", "Sue", false, 0},
		{"Sue", "Sueh", false, 0},
		{"Sue", "Suen", false, 0},
		{"Sue", "Sues", true, 97},
		{"Suez", "Sue", true, 97},
		{"Sue", "Susan", true, 100},
		{"S", "Su", true, 100},
		{"Su", "Suh", true, 96},
		// i1a and i1b: names with their own key.
		{"B", "Bo", true, 100},
		{"Bo", "B", true, 100},
		{"Bo", "Boh", true, 100},
		{"Bo", "Bow", true, 96},
		{"Bo", "Boe", true, 96},
		{"G", "Guy", true, 100},
		{"Guy", "Gu", true, 96},
		{"Guy", "Guye", true, 97},
		// i1a: alias-keyed names without a captured alias key.
		{"J", "Jo", false, 0},
		{"J", "Joe", false, 0},
		{"Jo", "Joe", true, 100},
		{"Joe", "Jo", true, 100},
		{"K", "Kay", false, 0},
		{"Kay", "Kai", false, 0},
		{"K", "Kai", true, 100},
		{"L", "Lou", false, 0},
		{"Lou", "Lu", false, 0},
		{"L", "Lu", true, 100},
		// fn1 row that must stay.
		{"T", "T1", true, 100},
		// Local contract, no native row: unchanged from origin/main.
		{"Ty", "Ty R", false, 0},
	} {
		match, confidence := fuzzyFirstName(c.a, c.b, false)
		if !match {
			confidence = 0
		}
		if got := math.Floor(confidence + 0.5); match != c.match || got != c.confidence {
			t.Errorf("custom first name %q/%q: %v %v, want %v %v", c.a, c.b, match, got, c.match, c.confidence)
		}
	}
	// Local contract: the standard First Name item keeps its own key for these
	// names, as on origin/main (TestFuzzyFirstAliasKeysStandardRows holds the
	// standard rule's native rows).
	for _, c := range []struct {
		a, b  string
		match bool
	}{
		{"Ty", "Tyh", false},
		{"Ty", "Tyw", false},
		{"Ty", "Tye", true},
		{"Tye", "Ty", true},
		{"Ty", "Tyr", false},
		{"Ray", "Ra", true},
		{"Ray", "Raym", false},
		{"Sue", "Su", true},
		{"Sue", "Sues", false},
	} {
		if match, _ := standardFirstName(c.a, c.b); match != c.match {
			t.Errorf("standard first name %q/%q: %v", c.a, c.b, match)
		}
	}
}
