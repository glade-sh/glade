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

// The fit rows already keep "Ty" from "Tyh" and "Tyw" (N4.FN.Typo.155), which
// Jaro-Winkler under a shared key would match as it matches "Bo" and "Boh":
// "Ty" has no match key of its own in a custom rule, so an initial does not
// find it either. Initials itself is unchanged for every other name.
func TestFuzzyFirstNameAliasKeyedNameHasNoInitial(t *testing.T) {
	for _, c := range []struct {
		a, b       string
		match      bool
		confidence float64
	}{
		// hf1 rows.
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
		// N4.FN.Typo.155 rows.
		{"Ty", "Tyh", false, 0},
		{"Tyw", "Ty", false, 0},
		// fn1 and N4.FN.Typo rows that must stay.
		{"T", "T1", true, 100},
		{"Bo", "Boh", true, 100},
		{"Bo", "Bow", true, 96},
		// Local contract, no native row: names without an alias key keep the
		// origin/main result.
		{"B", "Bo", true, 100},
		{"G", "Guy", true, 100},
		{"Ty", "Ty R", false, 0},
		{"Tye", "Tyeh", true, 97},
	} {
		match, confidence := fuzzyFirstName(c.a, c.b, false)
		if !match {
			confidence = 0
		}
		if got := math.Floor(confidence + 0.5); match != c.match || got != c.confidence {
			t.Errorf("custom first name %q/%q: %v %v, want %v %v", c.a, c.b, match, got, c.match, c.confidence)
		}
	}
	// Local contract: the standard rules keep their own key, so only the two
	// captured pairs stay apart there, as on origin/main.
	for _, c := range []struct {
		a, b  string
		match bool
	}{
		{"Ty", "Tyh", false},
		{"Ty", "Tyw", false},
		{"Ty", "Tye", true},
		{"Tye", "Ty", true},
	} {
		if match, _ := standardFirstName(c.a, c.b); match != c.match {
			t.Errorf("standard first name %q/%q: %v", c.a, c.b, match)
		}
	}
}
