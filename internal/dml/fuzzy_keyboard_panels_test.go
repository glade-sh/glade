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

// fuzzyPanelsTwinDir holds the blind native panels captured after the
// keyboard fallback (anonymous route only): the twin tk01-tk22, bk01 on and
// ck01 on.
const fuzzyPanelsTwinDir = "testdata/conformance/duplicate_fuzzy_panels/"

// fuzzyPanelsLastName returns the values and the shared native observation of
// each custom LastName panel in the given groups of duplicate_fuzzy_panels.
// Every API, route and repeat of a panel agrees.
func fuzzyPanelsLastName(t *testing.T, groups ...string) (map[string][]string, map[string]string) {
	t.Helper()
	return fuzzyPanelsLastNameFrom(t, fuzzyPanelsKeyboardDir, 8, groups...)
}

// fuzzyPanelsLastNameFrom is fuzzyPanelsLastName for another fixture
// directory; observations is the number of native rows each panel holds.
func fuzzyPanelsLastNameFrom(t *testing.T, dir string, observationsPerPanel int, groups ...string) (map[string][]string, map[string]string) {
	t.Helper()
	raw, err := os.ReadFile(dir + "cases.json")
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
	raw, err = os.ReadFile(dir + "org.tsv")
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
		if count[method] != observationsPerPanel {
			t.Fatalf("%s: %d native observations, want %d", method, count[method], observationsPerPanel)
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
// @IsTest, two repeats, every ordered pair of each panel. The blind twin
// (internal/dml/testdata/conformance/duplicate_fuzzy_panels, tk01-tk22) is the
// same rule on 65 other stems, anonymous route only. The keyboard fallback adds
// no match native does not report, on any of them, and every pair the scorer
// without the fallback gets right stays right. The fit panels (the keyboard
// group, k1 to k3) fitted the shape table and the per-shape floors that the
// single margin replaced; the hkeyboard group was in view when the shape rule
// was chosen, so it is not held-out evidence for the shape gate. The twin, the
// second and the third blind panel were captured after the code they score.
// The twin's tk14 sets the highest native non-match the margin clears; none of
// the three blind groups fitted the shape table.
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
		"k1":  {fuzzyNameKeysCount{native: 180, hit: 100, miss: 80, confidence: 10}, 934, 878},
		"k2":  {fuzzyNameKeysCount{native: 182, hit: 108, miss: 74, confidence: 8}, 942, 888},
		"k3":  {fuzzyNameKeysCount{native: 194, hit: 124, miss: 70, confidence: 14}, 940, 896},
		"hk1": {fuzzyNameKeysCount{native: 122, hit: 82, miss: 40, confidence: 8}, 628, 600},
		"hk2": {fuzzyNameKeysCount{native: 58, hit: 46, miss: 12, confidence: 6}, 238, 228},
		"bk":  {fuzzyNameKeysCount{native: 2056, hit: 1454, miss: 602, confidence: 350}, 5204, 4374},
		"tk":  {fuzzyNameKeysCount{native: 2151, hit: 1397, miss: 754, confidence: 212}, 14667, 14303},
		"ck":  {fuzzyNameKeysCount{native: 2087, hit: 1823, miss: 264, confidence: 404}, 5893, 4917},
	}
	// The blind panels are counted by group: tk (tk01 to tk22), bk and ck.
	for _, panel := range keyboardTwinPanels(t, "tkeyboard", "bkeyboard", "ckeyboard") {
		values[panel.name] = panel.values
		observations[panel.name] = ""
	}
	twin := map[string]map[[2]int]float64{}
	for _, panel := range keyboardTwinPanels(t, "tkeyboard", "bkeyboard", "ckeyboard") {
		twin[panel.name] = panel.native
	}
	rounded := func(matched bool, confidence float64) float64 {
		if !matched {
			return 0
		}
		return math.Floor(confidence + 0.5)
	}
	got := map[string]*tally{}
	for _, method := range slices.Sorted(maps.Keys(values)) {
		vals := values[method]
		var native map[[2]int]float64
		group := method
		if n, ok := twin[method]; ok {
			native, group = n, "tk"
			switch {
			case strings.HasPrefix(method, "bk"):
				group = "bk"
			case strings.HasPrefix(method, "ck"):
				group = "ck"
			}
		} else {
			native = fuzzyNameKeysPanel(t, observations[method], len(vals))
		}
		if got[group] == nil {
			got[group] = &tally{}
		}
		tl := got[group]
		for i, a := range vals {
			for j, b := range vals {
				n := native[[2]int{i, j}]
				g := rounded(fuzzyLastName(a, b, false))
				base := rounded(lastNameTrainBase(a, b))
				if g == n {
					tl.exact++
				}
				if base == n {
					tl.base++
				}
				switch {
				case n > 0 && g > 0:
					tl.native++
					tl.hit++
					if n != g {
						tl.confidence++
					}
				case n > 0:
					tl.native++
					tl.miss++
				case g > 0:
					tl.extra++
				}
				if n == 0 && g > 0 {
					t.Errorf("%s %s=>%s: Glade matches at %v, native has no match", method, a, b, g)
				}
				if base == n && g != n {
					t.Errorf("%s %s=>%s: Glade %v, native %v (the scorer without the fallback agrees with native)", method, a, b, g, n)
				}
			}
		}
	}
	for _, group := range slices.Sorted(maps.Keys(got)) {
		tl := *got[group]
		t.Logf("PANEL %s: %s exact=%d base=%d", group, tl.fuzzyNameKeysCount, tl.exact, tl.base)
		if tl != want[group] {
			t.Errorf("%s: %s exact=%d base=%d, want %s exact=%d base=%d", group, tl.fuzzyNameKeysCount, tl.exact, tl.base, want[group].fuzzyNameKeysCount, want[group].exact, want[group].base)
		}
	}
}
