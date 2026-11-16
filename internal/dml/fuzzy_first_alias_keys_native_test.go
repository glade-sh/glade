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

// NATIVE fuzzy_first_alias_keys/org.tsv:2-17 (i1a, i1b): a custom FirstName
// rule at API 62.0 and 67.0, anonymous and @IsTest, two repeats. Every
// ordered pair goes through the shared duplicate-rule matcher and must agree
// exactly. "Ty" finds "Tyr" at 96 and no other name: not "Tye", "Tyh", "Tyw",
// "Tyl", "Tya" or "T". "Ray" finds "Raym" at 97 and neither "Rae", "Raye",
// "Ra" nor "R"; "Sue" finds "Sues" and "Suez" at 97 and neither "Su" nor "S".
// "Bo" and "Guy" keep their own key: "B" finds "Bo", "Bo" finds "Boe".
func TestFuzzyFirstAliasKeysNativeRows(t *testing.T) {
	cases, observations := fuzzyNameKeysDataFrom(t, fuzzyFirstAliasKeysDir)
	if len(cases) != 4 {
		t.Fatalf("cases %d", len(cases))
	}
	want := map[string]fuzzyNameKeysCount{
		"i1a": {native: 72, hit: 72},
		"i1b": {native: 90, hit: 90},
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

// baseFirstNameGate is the first-word gate of fuzzyFirstName before the
// custom alias keys: the names are compared when it reports true.
func baseFirstNameGate(a, b string) bool {
	fx, fy := foldLowered(strings.ToLower(strings.TrimSpace(a))), foldLowered(strings.ToLower(strings.TrimSpace(b)))
	wx, wy := firstNameWord(fx), firstNameWord(fy)
	if wx == wy {
		return true
	}
	kx, ky := firstNameKey(wx), firstNameKey(wy)
	return kx != "" && kx == ky && !nameKeptApart[orderedPair(fx, fy)]
}

// The custom alias keys come from single-word panels (i1a, i1b). No row
// holds a name of several words or one with punctuation, so those shapes keep
// the gate they had before the keys: "Ty Ann" still finds "Tye Ann".
func TestFuzzyFirstAliasKeysOtherShapesKeepBase(t *testing.T) {
	matches := [][2]string{{"Ty Ann", "Tye Ann"}, {"Ray Ann", "Raye Ann"}, {"Sue Ann", "Su Ann"}}
	for _, pair := range matches {
		for _, p := range [][2]string{pair, {pair[1], pair[0]}} {
			if ok, _ := fuzzyFirstName(p[0], p[1], false); !ok {
				t.Errorf("%q=>%q: no match, base matches", p[0], p[1])
			}
		}
	}
	if ok, confidence := fuzzyFirstName("Ty Ann", "Tye Ann", false); !ok || math.Abs(confidence-98.10) > 0.01 {
		t.Errorf("Ty Ann=>Tye Ann: %v %.2f, base true 98.10", ok, confidence)
	}
	heads := []string{"Ty", "Tye", "Tyr", "Tyl", "T", "Ray", "Raye", "Raym", "Rayma", "Rae", "R", "Sue", "Su", "Sues", "Suesa", "S"}
	var names []string
	for _, head := range heads {
		names = append(names, head+" Ann", head+" A", head+"-Ann", head+".", head+" Ann Lee")
	}
	all := append(append([]string{}, heads...), names...)
	for _, a := range names {
		for _, b := range all {
			for _, p := range [][2]string{{a, b}, {b, a}} {
				gate := baseFirstNameGate(p[0], p[1])
				ok, _ := fuzzyFirstName(p[0], p[1], false)
				if !gate && ok {
					t.Errorf("%q=>%q: match, base gate keeps them apart", p[0], p[1])
				}
				if gate {
					fx, fy := foldLowered(strings.ToLower(p[0])), foldLowered(strings.ToLower(p[1]))
					want := fx == fy || nameVariantOf(fx, fy) || firstNameInitials(fx, fy) || firstNameInitials(fy, fx) ||
						jaroWinkler(strings.ToLower(p[0]), strings.ToLower(p[1])) >= firstNameJaroWinkler-1e-9
					if ok != want {
						t.Errorf("%q=>%q: match %v, base %v", p[0], p[1], ok, want)
					}
				}
			}
		}
	}
}

// Single-word neighbors no row holds follow the nearest rows: a name under
// the alias key matches as "Raym", "Sues" and "Tyr" do, a name under the
// spelled key stays apart as "Rae", "Su" and "Tye" do.
func TestFuzzyFirstAliasKeysSingleWordNeighbors(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"Ray", "Rayma", true}, {"Sue", "Suesa", true},
		{"Ray", "Raya", false}, {"Sue", "Suey", false}, {"Ty", "Tyy", false},
	} {
		if ok, _ := fuzzyFirstName(c.a, c.b, false); ok != c.want {
			t.Errorf("%q=>%q: match %v, want %v", c.a, c.b, ok, c.want)
		}
	}
}
