package dml

import (
	"math"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

const fuzzyFirstAliasKeysDir = "testdata/conformance/fuzzy_first_alias_keys/"

// fuzzyFirstAliasKeysCompare counts one panel: native and got hold the
// rounded confidence of every ordered pair, 0 for no match.
func fuzzyFirstAliasKeysCompare(values []string, native map[[2]int]float64, got func(i, j int) float64, report func(i, j int, native, got float64)) fuzzyNameKeysCount {
	var counts fuzzyNameKeysCount
	for i := range values {
		for j := range values {
			n, g := native[[2]int{i, j}], got(i, j)
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
			if report != nil {
				report(i, j, n, g)
			}
		}
	}
	return counts
}

// NATIVE fuzzy_first_alias_keys/org.tsv:2-17 (i1a, i1b), 34-41 (i1c): a custom FirstName
// rule at API 62.0 and 67.0, anonymous and @IsTest, two repeats. Every
// ordered pair goes through the shared duplicate-rule matcher and must agree
// exactly. "Ty" finds "Tyr" at 96 and no other name: not "Tye", "Tyh", "Tyw",
// "Tyl", "Tya" or "T". "Ray" finds "Raym" at 97 and neither "Rae", "Raye",
// "Ra" nor "R"; "Sue" finds "Sues" and "Suez" at 97 and neither "Su" nor "S".
// "Bo" and "Guy" keep their own key: "B" finds "Bo", "Bo" finds "Boe".
func TestFuzzyFirstAliasKeysNativeRows(t *testing.T) {
	cases, observations := fuzzyNameKeysDataFrom(t, fuzzyFirstAliasKeysDir)
	if len(cases) != 5 {
		t.Fatalf("cases %d", len(cases))
	}
	want := map[string]fuzzyNameKeysCount{
		"i1a": {native: 72, hit: 72},
		"i1b": {native: 90, hit: 90},
		// org.tsv:34-41: the alias-keyed names as the first word of a longer
		// name. The key follows the first word: "Ty Ann" finds "Tyr Ann" at 98
		// and neither "Tye Ann" nor "T Ann"; "Ray Ann" finds "Raym Ann" and
		// not "Raye Ann" or "Raya"; "Sue Ann" finds "Sues Ann" and not
		// "Su Ann". "Ray" finds "Rayma" and "Sue" finds "Suesa" at 95. An
		// initial alone finds a longer name under its key at 100 ("T." finds
		// "Tye Ann"); "T Ann" finds "Tye Ann" by Jaro-Winkler at 96.
		"i1c": {native: 94, hit: 94},
	}
	for name, total := range want {
		c := cases[name]
		if c.Kind != "panel" || c.Rule != "N1First" {
			t.Fatalf("%s: case %+v", name, c)
		}
		native := fuzzyNameKeysPanel(t, observations[name], len(c.Values))
		org, ids := fuzzyNameKeysOrg(c, "Allow", c.Values...)
		index := map[storage.ID]int{}
		for i, id := range ids {
			index[id] = i
		}
		rule := fuzzyNameKeysRule(c, "Allow")
		got := make([]map[int]float64, len(c.Values))
		for i, value := range c.Values {
			got[i] = map[int]float64{}
			result, supported := FindDuplicateMatches(org, rule, fuzzyNameKeysRecord(c, value), nil)
			if !supported {
				t.Fatal("rule not supported")
			}
			for _, match := range result.Matches {
				for k, record := range match.Records {
					got[i][index[record.ID]] = math.Floor(match.Confidences[k] + 0.5)
				}
			}
		}
		counts := fuzzyFirstAliasKeysCompare(c.Values, native, func(i, j int) float64 { return got[i][j] }, func(i, j int, n, g float64) {
			t.Errorf("%s/%s=>%s: native=%v glade=%v", name, c.Values[i], c.Values[j], n, g)
		})
		t.Logf("PANEL %s: %s", name, counts)
		if counts != total {
			t.Errorf("%s counts %s, want %s", name, counts, total)
		}
	}
}

// fuzzyFirstAliasKeysStandardFloor is what the standard Contact rule
// reproduces of its two panels today: pairs found, and pairs found at the
// native confidence. Natively every name of a panel that shares the first
// letter matches under one last name and email address ("Ty" finds "Tye",
// "Tyh" and "Tyw" at 100, "Ray" finds "Rae" at 100 and "Raym" at 98); the
// standard rule's first-name scoring is not this fixture's subject. A count
// may rise, never fall, and Glade reports no pair that native does not.
var fuzzyFirstAliasKeysStandardFloor = map[string][3]int{
	// native pairs, found, found at the native confidence
	"i1sa": {120, 90, 78},
	"i1sb": {216, 128, 96},
}

// NATIVE fuzzy_first_alias_keys/org.tsv:18-33 (i1sa, i1sb): the standard
// Contact rule at API 62.0 and 67.0, anonymous and @IsTest, two repeats; the
// same values as i1a and i1b under one last name and email address. The
// custom-rule alias keys must not reach the standard rule: its counts stay
// at or above the floor measured before them.
func TestFuzzyFirstAliasKeysStandardRows(t *testing.T) {
	t.Setenv("GLADE_STANDARD_DUPLICATE_RULES", "on")
	cases, observations := fuzzyNameKeysDataFrom(t, fuzzyFirstAliasKeysDir)
	for name, floor := range fuzzyFirstAliasKeysStandardFloor {
		c := cases[name]
		if c.Kind != "panel" || c.Rule != "Standard_Contact_Duplicate_Rule" {
			t.Fatalf("%s: case %+v", name, c)
		}
		native := fuzzyNameKeysPanel(t, observations[name], len(c.Values))
		records := make([]storage.Record, len(c.Values))
		for i, value := range c.Values {
			records[i] = fuzzyNameKeysRecord(c, value)
		}
		counts := fuzzyFirstAliasKeysCompare(c.Values, native, func(i, j int) float64 {
			match, confidence := standardDuplicateMatcher(nil, c.Object, c.Object, records[i])(records[j])
			if !match {
				return 0
			}
			return math.Floor(confidence + 0.5)
		}, func(i, j int, n, g float64) {
			a, b, _ := strings.Cut(c.Values[i], "|")
			b, _, _ = strings.Cut(c.Values[j], "|")
			if n == 0 {
				t.Errorf("%s/%s=>%s: glade=%v, native has no match", name, a, b, g)
			} else {
				t.Logf("%s/%s=>%s: native=%v glade=%v", name, a, b, n, g)
			}
		})
		t.Logf("PANEL %s: %s", name, counts)
		if exact := counts.hit - counts.confidence; counts.native != floor[0] || counts.hit < floor[1] || exact < floor[2] || counts.extra != 0 {
			t.Errorf("%s counts %s, want native=%d hit>=%d exact>=%d extra=0", name, counts, floor[0], floor[1], floor[2])
		}
	}
}
