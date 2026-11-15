package dml

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE fuzzy_name_keys/org.tsv (suf1, rsp1, ks1) and
// duplicate_fuzzy_matching/org.tsv (P4.LN.KB1, P4.LN.KB2): Keyboard Distance
// uses positional substitutions and trailing characters; the final position
// is cheaper. Its caller limits the fallback to captured joint shapes.
func TestKeyboardDistanceNativeShapes(t *testing.T) {
	for _, c := range []struct {
		a, b       string
		match      bool
		confidence float64
	}{
		{"Harlow II", "Harlow IV", true, 98},         // suf1: native 97; preserve the base score
		{"Harlow Jr", "Harlowe", true, 97},           // suf1: a space against a letter, two trailing; native 96 (fuzzyNameKeysResiduals)
		{"Farnsdelwick", "Farnsdraeovk", true, 96},   // ks1: five substitutions
		{"Abernathy", "Abernaty", true, 98},          // rsp1: h/y and a trailing y; native 99 (fuzzyNameKeysResiduals)
		{"Campbell", "Cambell", false, 0},            // rsp1
		{"Richardson", "Richardsop", true, 98},       // P4.LN.KB1
		{"Martinez", "Martinezk", true, 99},          // P4.LN.KB2
		{"Martinez", "Martinezkmp", true, 97},        // P4.LN.KB2
		{"Frank", "Franklin", false, 0},              // Probe
		{"Garcia", "García", true, 99},               // accent
		{"Morvenhsaapw", "Morvenlfddww", false, 0},   // ks2 non-match
		{"Quenderthwait", "Quenderqdijit", false, 0}, // ks3 non-match
		{"Quenderthwait", "Quenderiltait", true, 97}, // ks3
		{"Morvenlfddow", "Morvenjsaapw", true, 97},   // ks2
	} {
		got, confidence := fuzzyLastName(c.a, c.b, false)
		if rounded := math.Floor(confidence + 0.5); got != c.match || got && rounded != c.confidence {
			t.Errorf("%q/%q: match=%v confidence=%v, want %v %v", c.a, c.b, got, confidence, c.match, c.confidence)
		}
		if back, _ := fuzzyLastName(c.b, c.a, false); back != got {
			t.Errorf("%q/%q: not symmetric", c.a, c.b)
		}
	}
}

// Local contract: characters no row measures never substitute, a trailing
// digit keeps the earlier cost for one or two characters only, and the byte
// and rune paths agree.
func TestKeyboardDistanceUnmeasuredShapes(t *testing.T) {
	if _, ok := keyboardDistance("harlow 2", "harlow 3"); ok {
		t.Error("a digit substituted")
	}
	if _, ok := keyboardDistance("harløw", "harlow"); ok {
		t.Error("a letter that is not on the keyboard substituted")
	}
	if score, ok := keyboardDistance("harlowe", "harlowe12"); !ok || math.Abs(score-100*(1-0.5/9)) > 1e-9 {
		t.Errorf("two trailing digits: %v %v", score, ok)
	}
	if _, ok := keyboardDistance("harlowe", "harlowe123"); ok {
		t.Error("three trailing digits matched")
	}
	for _, pair := range [][2]string{{"farnsdelwick", "farnsdraeovk"}, {"harlow jr", "harlowe"}, {"martinez", "martinezkmp"}, {"", "ab"}} {
		ascii, okASCII := keyboardDistance(pair[0], pair[1])
		runes, okRunes := keyboardDistanceRunes([]rune(pair[0]), []rune(pair[1]))
		if ascii != runes || okASCII != okRunes {
			t.Errorf("%q/%q: bytes %v %v, runes %v %v", pair[0], pair[1], ascii, okASCII, runes, okRunes)
		}
	}
	// The standard rules keep the earlier costs.
	if matched, _ := standardLastName("Farnsdelwick", "Farnsdraeovk"); matched {
		t.Error("standard Last Name matched five substitutions")
	}
}

// The legacy scorer is the contract for every equal-length single substitution.
func TestLastNameSingleSubstitutionMatchesLegacy(t *testing.T) {
	for _, name := range []string{"scott", "richardsoe", "bradlet", "farnsdelwick", "harlow ii", "garcía"} {
		original := []rune(name)
		for i, old := range original {
			for _, replacement := range []rune("abcdefghijklmnopqrstuvwxyz -é") {
				if old == replacement {
					continue
				}
				changed := append([]rune(nil), original...)
				changed[i] = replacement
				a, b := name, string(changed)
				x, y := foldText(a), foldText(b)
				rx, ry := []rune(x), []rune(y)
				if len(rx) != len(ry) {
					continue
				}
				differences := 0
				for i := range rx {
					if rx[i] != ry[i] {
						differences++
					}
				}
				if differences != 1 {
					continue
				}
				var want bool
				var confidence float64
				switch {
				case x == y:
					want, confidence = true, 100
				case lastNameWordKey(x) != lastNameWordKey(y):
				default:
					x, y = lastNameApostrophes.Replace(x), lastNameApostrophes.Replace(y)
					if x == y || lastNameMetaphone3(x) == lastNameMetaphone3(y) {
						want, confidence = true, 100
					} else {
						want, confidence = lastNameKeyboard(x, y)
					}
				}
				got, score := fuzzyLastName(a, b, false)
				if got != want || score != confidence {
					t.Errorf("%q/%q: got %v %v; legacy %v %v", a, b, got, score, want, confidence)
				}
			}
		}
	}
}

// Every pair accepted by the original keyboard scorer retains its exact score.
func TestLastNamePreviouslyMatchedScores(t *testing.T) {
	cases, _ := fuzzyNameKeysData(t)
	all := map[string][]string{}
	for name, c := range cases {
		if c.Kind == "panel" && c.Item == "LastName" {
			all[name] = c.Values
		}
	}
	raw, err := os.ReadFile(filepath.Join(fuzzyFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 {
			t.Fatal("invalid row")
		}
		var row fuzzyNativeRow
		if err := json.Unmarshal([]byte(cells[3]), &row); err != nil {
			t.Fatal(err)
		}
		if row.Field != "LastName" || row.Method != "LastName" {
			continue
		}
		for _, v := range row.Values {
			if v != nil {
				all[cells[0]+"/"+cells[2]] = append(all[cells[0]+"/"+cells[2]], *v)
			}
		}
	}
	for name, values := range all {
		for _, a := range values {
			for _, b := range values {
				x, y := foldText(a), foldText(b)
				if x == y || lastNameWordKey(x) != lastNameWordKey(y) {
					continue
				}
				x, y = lastNameApostrophes.Replace(x), lastNameApostrophes.Replace(y)
				if x == y || lastNameMetaphone3(x) == lastNameMetaphone3(y) {
					continue
				}
				want, score := lastNameKeyboard(x, y)
				if !want {
					continue
				}
				got, actual := fuzzyLastName(a, b, false)
				if got != want || actual != score {
					t.Errorf("%s %q/%q: got %v %v; base %v %v", name, a, b, got, actual, want, score)
				}
			}
		}
	}
}

func TestLastNameWordKeyCacheSeparatesInputs(t *testing.T) {
	// The whole folded name is the only variable input to the cached key.
	for _, c := range []struct{ input, want string }{
		{"harlow 1", "#1"},
		{"harlow 2", "#2"},
		{"harlow ii", lastNameCode("harlow")},
		{"smith ii", lastNameCode("smith")},
	} {
		for i := 0; i < 2; i++ {
			if got := lastNameWordKey(c.input); got != c.want {
				t.Errorf("%q: got %q, want %q", c.input, got, c.want)
			}
		}
	}
}

// keyboardBase is the origin/main Last Name result after the exact, key and
// Metaphone 3 steps: the keyboard scorer the standard rules still use.
func keyboardBase(t *testing.T, a, b string) (bool, float64) {
	t.Helper()
	x, y := foldText(a), foldText(b)
	if x == y || lastNameWordKey(x) != lastNameWordKey(y) {
		t.Fatalf("%q/%q does not reach keyboard scoring", a, b)
	}
	x, y = lastNameApostrophes.Replace(x), lastNameApostrophes.Replace(y)
	if x == y || lastNameMetaphone3(x) == lastNameMetaphone3(y) {
		t.Fatalf("%q/%q does not reach keyboard scoring", a, b)
	}
	return lastNameKeyboard(x, y)
}

// Differential against origin/main ad1577dbc6: each pair has a shape no
// native row holds, the new scorer alone would match it, and the result stays
// the literal origin/main score. The first four are outside the captured
// extrema; the rest are holes inside them.
func TestLastNameKeyboardUncapturedShapesKeepBase(t *testing.T) {
	for _, pair := range []struct {
		a, b  string
		shape [3]int
		score float64
	}{
		{"Farnsdelwickaaaa", "Farnsdraeovkaaaa", [3]int{16, 5, 0}, 97.625},
		{"Farnsdelwick", "Farnsdraeovkzzzzzzzzzz", [3]int{22, 5, 10}, 92.590909090909093},
		{"erkecpmeitrn", "erkqudnukxik", [3]int{12, 9, 0}, 91.916666666666671},
		{"ab", "ap", [3]int{2, 1, 0}, 88},
		{"Farnsdelwickab", "Farnsdraeovkab", [3]int{14, 5, 0}, 97.28571428571428},
		{"Farnsdelwickabc", "Farnsdraeovkabc", [3]int{15, 5, 0}, 97.46666666666667},
		{"Farnsdelwickam", "Farnsdraeovka", [3]int{14, 5, 1}, 96.39285714285714},
		{"Quenderthwaits", "Quenderiltaits", [3]int{14, 3, 0}, 98.28571428571428},
		{"Quenderthwaite", "Quenderiltait", [3]int{14, 3, 1}, 97.39285714285714},
	} {
		x, y := lastNameApostrophes.Replace(foldText(pair.a)), lastNameApostrophes.Replace(foldText(pair.b))
		if got := keyboardShape(x, y); got != pair.shape {
			t.Errorf("%q/%q: shape %v, want %v", pair.a, pair.b, got, pair.shape)
		}
		if keyboardPanelShape(x, y) {
			t.Errorf("%q/%q: shape %v is captured", pair.a, pair.b, pair.shape)
		}
		want, score := keyboardBase(t, pair.a, pair.b)
		got, actual := fuzzyLastName(pair.a, pair.b, false)
		if want || score != pair.score || got != want || actual != score {
			t.Errorf("%q/%q: got %v %v, base %v %v, origin/main false %v", pair.a, pair.b, got, actual, want, score, pair.score)
		}
		// Without the gate the result would change.
		if raw, ok := keyboardDistance(x, y); (ok && raw >= 90) == want && averageWith100(raw) == score {
			t.Errorf("%q/%q: the new scorer agrees with base; the row proves nothing", pair.a, pair.b)
		}
	}
}

// Every uncaptured shape keeps the base result: generated pairs sweep lengths
// to 17, substitutions to 9 and trailing counts to 10. Substitutions run
// back from the end of the shorter name so the match key survives.
func TestLastNameKeyboardUncapturedSweepKeepsBase(t *testing.T) {
	const tail = "ertuiopasdf"
	neighbor := strings.NewReplacer("a", "s", "b", "n", "c", "v", "d", "f", "e", "r", "f", "g", "g", "h", "h", "j", "i", "o", "k", "l",
		"l", "k", "m", "n", "n", "m", "o", "p", "p", "o", "r", "t", "s", "d", "t", "y", "u", "i", "v", "b", "w", "e", "y", "u", "z", "x", "j", "k", "q", "w", "x", "c")
	captured, checked, differs := map[[3]int]bool{}, map[[3]int]bool{}, 0
	for _, stem := range []string{"farnsdelwickmorvn", "quenderthwaitsbly", "morvenlfddowhtkgs", "bartholomewsdfghj"} {
		for length := 2; length <= len(stem); length++ {
			for trailing := 0; trailing <= 10 && trailing < length; trailing++ {
				short := length - trailing
				for substitutions := 0; substitutions <= 9 && substitutions <= short; substitutions++ {
					x := stem[:short]
					y := x[:short-substitutions] + neighbor.Replace(x[short-substitutions:]) + tail[:trailing]
					shape := keyboardShape(x, y)
					if shape != [3]int{length, substitutions, trailing} {
						t.Fatalf("%q/%q: shape %v", x, y, shape)
					}
					if x == y || lastNameWordKey(x) != lastNameWordKey(y) || lastNameMetaphone3(x) == lastNameMetaphone3(y) {
						continue
					}
					if keyboardPanelShape(x, y) {
						captured[shape] = true
						continue
					}
					checked[shape] = true
					want, score := lastNameKeyboard(x, y)
					if raw, ok := keyboardDistance(x, y); (ok && raw >= 90) != want || averageWith100(raw) != score {
						differs++
					}
					for _, order := range [][2]string{{x, y}, {y, x}} {
						if got, actual := fuzzyLastName(order[0], order[1], false); got != want || actual != score {
							t.Errorf("%q/%q: got %v %v, base %v %v", order[0], order[1], got, actual, want, score)
						}
					}
				}
			}
		}
	}
	t.Logf("%d captured and %d uncaptured shapes; the new scorer differs on %d uncaptured pairs", len(captured), len(checked), differs)
	if len(captured) < 90 || len(checked) < 300 || differs < 900 {
		t.Fatalf("sweep reached %d captured and %d uncaptured shapes, %d differing", len(captured), len(checked), differs)
	}
}

// keyboardShape is (length, substitutions, trailing) of two prepared names.
func keyboardShape(x, y string) [3]int {
	rx, ry := []rune(x), []rune(y)
	if len(rx) > len(ry) {
		rx, ry = ry, rx
	}
	diff := 0
	for j := range rx {
		if rx[j] != ry[j] {
			diff++
		}
	}
	return [3]int{len(ry), diff, len(ry) - len(rx)}
}

// The gate table is exactly the shapes of the captured pairs that reach the
// fallback: not an exact match, same key, different Metaphone 3 code, more
// than a single substitution, no terminal swap, and no base keyboard match.
func TestLastNameKeyboardPanelCells(t *testing.T) {
	cases, _ := fuzzyNameKeysData(t)
	var want [16][9]uint16
	measure := func(values []string) {
		for i, a := range values {
			for _, b := range values[i+1:] {
				x, y := foldText(a), foldText(b)
				if x == y || lastNameWordKey(x) != lastNameWordKey(y) {
					continue
				}
				x, y = lastNameApostrophes.Replace(x), lastNameApostrophes.Replace(y)
				if x == y || lastNameMetaphone3(x) == lastNameMetaphone3(y) || singleKeyboardSubstitution(x, y) {
					continue
				}
				if _, _, swap := lastNameTerminalSwap(x, y); swap {
					continue
				}
				if matched, _ := lastNameKeyboard(x, y); matched {
					continue
				}
				shape := keyboardShape(x, y)
				want[shape[0]][shape[1]] |= 1 << uint(shape[2])
			}
		}
	}
	for _, panel := range cases {
		if panel.Kind == "panel" && panel.Item == "LastName" {
			measure(panel.Values)
		}
	}
	raw, err := os.ReadFile(filepath.Join(fuzzyFixture, "org.tsv"))
	if err != nil {
		t.Fatal(err)
	}
	for _, line := range strings.Split(strings.TrimSpace(string(raw)), "\n")[1:] {
		cells := strings.SplitN(line, "\t", 4)
		if len(cells) != 4 {
			t.Fatal("invalid native row")
		}
		var row fuzzyNativeRow
		if err := json.Unmarshal([]byte(cells[3]), &row); err != nil {
			t.Fatal(err)
		}
		if row.Field != "LastName" || row.Method != "LastName" {
			continue
		}
		var values []string
		for _, value := range row.Values {
			if value != nil {
				values = append(values, *value)
			}
		}
		measure(values)
	}
	if want != keyboardPanelCells {
		var b strings.Builder
		for length, row := range want {
			if row != [9]uint16{} {
				fmt.Fprintf(&b, "\t%d: {", length)
				for i, bits := range row {
					if i > 0 {
						b.WriteString(", ")
					}
					fmt.Fprintf(&b, "%#04x", bits)
				}
				b.WriteString("},\n")
			}
		}
		t.Fatalf("captured shapes changed; keyboardPanelCells should be:\n%s", b.String())
	}
}

func TestKeyboardPanelShapeBoundaries(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"abcd", "abxy", true},                                   // (4, 2, 0)
		{"éééé", "ééab", true},                                   // (4, 2, 0) in runes
		{"abc", "abd", false},                                    // (3, 1, 0): a single substitution never reaches the gate
		{"abc", "axy", false},                                    // (3, 2, 0)
		{"éé", "éa", false},                                      // (2, 1, 0)
		{"abcdefghijklmno", "abcdefghi", true},                   // (15, 0, 6)
		{"abcdefghijklmno", "abcdefghijklmnp", false},            // (15, 1, 0)
		{"abcdefghijklmnop", "abcdefghijklmnoq", false},          // length 16
		{"abcdefghi", "zzzzzzzzZ", false},                        // nine substitutions
		{"abc", "abcdefghijklM", false},                          // ten trailing
		{"farnsdelwicka", "farnsdraeovka", true},                 // (13, 5, 0): 60 captured pairs hold this shape
		{"abcdefghijklm", "abcdefgzzzzzz", true},                 // (13, 6, 0)
		{"abcdefghijklmn", "abcdefgzzzzzzn", false},              // (14, 6, 0)
		{"abcdefghijklm", "abcdefgzzzzzzzzzzzzzzzzzzzzz", false}, // trailing beyond the table's bits
	} {
		if got := keyboardPanelShape(c.a, c.b); got != c.want {
			t.Errorf("%q/%q: %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
