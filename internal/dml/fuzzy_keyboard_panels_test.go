package dml

import (
	"encoding/json"
	"maps"
	"math"
	"os"
	"regexp"
	"slices"
	"strings"
	"testing"
)

const fuzzyPanelsKeyboardDir = "../apextest/testdata/conformance/duplicate_fuzzy_panels/"

// fuzzyPanelsLastName returns the values and the shared native observation of
// each custom LastName panel in the given groups of duplicate_fuzzy_panels.
// Every API, route and repeat of a panel agrees.
func fuzzyPanelsLastName(t *testing.T, groups ...string) (map[string][]string, map[string]string) {
	t.Helper()
	raw, err := os.ReadFile(fuzzyPanelsKeyboardDir + "cases.json")
	if err != nil {
		t.Fatal(err)
	}
	var cases map[string]map[string]string
	if err := json.Unmarshal(raw, &cases); err != nil {
		t.Fatal(err)
	}
	quoted := regexp.MustCompile(`'([^'\\]*)'`)
	values := map[string][]string{}
	for _, group := range groups {
		if len(cases[group]) == 0 {
			t.Fatalf("no panels in group %s", group)
		}
		for method, source := range cases[group] {
			head, list, ok := strings.Cut(source, "new List<String>{")
			if !ok || !strings.Contains(head, "'Contact', 'G4Last', 'LastName'") {
				t.Fatalf("%s: not a LastName panel", method)
			}
			for _, m := range quoted.FindAllStringSubmatch(list, -1) {
				values[method] = append(values[method], m[1])
			}
		}
	}
	raw, err = os.ReadFile(fuzzyPanelsKeyboardDir + "org.tsv")
	if err != nil {
		t.Fatal(err)
	}
	observations := map[string]string{}
	count := map[string]int{}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		f := strings.Split(line, "\t")
		if len(f) != 6 || !slices.Contains(groups, f[3]) {
			continue
		}
		if prior, ok := observations[f[4]]; ok && prior != f[5] {
			t.Fatalf("%s: native observations disagree", f[4])
		}
		observations[f[4]] = f[5]
		count[f[4]]++
	}
	for method := range values {
		if count[method] != 8 {
			t.Fatalf("%s: %d native observations, want 8", method, count[method])
		}
	}
	return values, observations
}

// lastNameTrainBase is the custom Last Name scorer without the keyboard
// fallback: the match key, then the steps the standard rules share.
func lastNameTrainBase(a, b string) (bool, float64) {
	x, y := foldText(a), foldText(b)
	if x == y {
		return true, 100
	}
	if lastNameWordKey(x) != lastNameWordKey(y) {
		return false, 0
	}
	return lastNameAlgorithms(x, y)
}

// NATIVE duplicate_fuzzy_panels/org.tsv:34-57,130-145 (groups keyboard and
// hkeyboard): a custom LastName rule at API 62.0 and 67.0, anonymous and
// @IsTest, two repeats, every ordered pair of each panel. The keyboard
// fallback adds no match native does not report, and every pair the scorer
// without the fallback gets right stays right. The keyboard group sets the
// fallback floors (TestLastNameKeyboardPanelFloors); the hkeyboard group,
// captured as a held-out run, sets nothing and has no extra match either.
func TestLastNameKeyboardFuzzyPanelsNativeRows(t *testing.T) {
	values, observations := fuzzyPanelsLastName(t, "keyboard", "hkeyboard")
	// exact counts the ordered pairs (self pairs included) where Glade and
	// native agree on the match and its confidence; base is the same count
	// without the keyboard fallback.
	type tally struct {
		fuzzyNameKeysCount
		exact, base int
	}
	want := map[string]tally{
		"k1":  {fuzzyNameKeysCount{native: 180, hit: 156, miss: 24, confidence: 22}, 978, 878},
		"k2":  {fuzzyNameKeysCount{native: 182, hit: 84, miss: 98, confidence: 4}, 922, 888},
		"k3":  {fuzzyNameKeysCount{native: 194, hit: 154, miss: 40, confidence: 18}, 966, 896},
		"hk1": {fuzzyNameKeysCount{native: 122, hit: 62, miss: 60, confidence: 4}, 612, 600},
		"hk2": {fuzzyNameKeysCount{native: 58, hit: 52, miss: 6, confidence: 8}, 242, 228},
	}
	rounded := func(matched bool, confidence float64) float64 {
		if !matched {
			return 0
		}
		return math.Floor(confidence + 0.5)
	}
	for _, method := range slices.Sorted(maps.Keys(values)) {
		vals := values[method]
		native := fuzzyNameKeysPanel(t, observations[method], len(vals))
		var got tally
		for i, a := range vals {
			for j, b := range vals {
				n := native[[2]int{i, j}]
				g := rounded(fuzzyLastName(a, b, false))
				base := rounded(lastNameTrainBase(a, b))
				if g == n {
					got.exact++
				}
				if base == n {
					got.base++
				}
				switch {
				case n > 0 && g > 0:
					got.native++
					got.hit++
					if n != g {
						got.confidence++
					}
				case n > 0:
					got.native++
					got.miss++
				case g > 0:
					got.extra++
				}
				if n == 0 && g > 0 {
					t.Errorf("%s %s=>%s: Glade matches at %v, native has no match", method, a, b, g)
				}
				if base == n && g != n {
					t.Errorf("%s %s=>%s: Glade %v, native %v (the scorer without the fallback agrees with native)", method, a, b, g, n)
				}
			}
		}
		t.Logf("PANEL %s: %s exact=%d base=%d", method, got.fuzzyNameKeysCount, got.exact, got.base)
		if got != want[method] {
			t.Errorf("%s: %s exact=%d base=%d, want %s exact=%d base=%d", method, got.fuzzyNameKeysCount, got.exact, got.base, want[method].fuzzyNameKeysCount, want[method].exact, want[method].base)
		}
	}
}
