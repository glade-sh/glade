package dml

import (
	"encoding/json"
	"fmt"
	"maps"
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"unicode/utf8"
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
		{"Farnsdelwick", "Farnsdraeovk", false, 0},   // ks1: native matches at 96; the fitted score is 92.07, below the fallback margin
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

// wantLastNameFallback is the result the Last Name fallback gives two prepared
// names that reach it from a shape no native row holds: the base result,
// unless the base scorer misses and the flat Keyboard Distance reaches keyboardFallbackMargin.
func wantLastNameFallback(t *testing.T, x, y string) (bool, float64) {
	t.Helper()
	if matched, confidence := keyboardBase(t, x, y); matched {
		return matched, confidence
	}
	if score, ok := keyboardFlatDistance(x, y); ok && score >= keyboardFallbackMargin {
		return true, averageWith100(score)
	}
	return keyboardBase(t, x, y)
}

// The flat score is not gated by shape. Each pair has a shape no native row
// holds: where the flat score reaches keyboardFallbackMargin the pair matches
// at (100 + score) / 2, and where it does not the result stays the literal
// origin/main ad1577dbc6 score. The first four are outside the captured
// extrema; the rest are holes inside them.
func TestLastNameKeyboardUncapturedShapesUseFlatScore(t *testing.T) {
	for _, pair := range []struct {
		a, b  string
		shape [3]int
		score float64
		match bool
	}{
		{"Farnsdelwickaaaa", "Farnsdraeovkaaaa", [3]int{16, 5, 0}, 97.625, true},
		{"Farnsdelwick", "Farnsdraeovkzzzzzzzzzz", [3]int{22, 5, 10}, 92.590909090909093, false},
		{"erkecpmeitrn", "erkqudnukxik", [3]int{12, 9, 0}, 91.916666666666671, false},
		{"ab", "ap", [3]int{2, 1, 0}, 88, false},
		{"Farnsdelwickab", "Farnsdraeovkab", [3]int{14, 5, 0}, 97.28571428571428, true},
		{"Farnsdelwickabc", "Farnsdraeovkabc", [3]int{15, 5, 0}, 97.46666666666667, true},
		{"Farnsdelwickam", "Farnsdraeovka", [3]int{14, 5, 1}, 96.39285714285714, false},
		{"Quenderthwaits", "Quenderiltaits", [3]int{14, 3, 0}, 98.28571428571428, true},
		{"Quenderthwaite", "Quenderiltait", [3]int{14, 3, 1}, 97.39285714285714, true},
	} {
		x, y := lastNameApostrophes.Replace(foldText(pair.a)), lastNameApostrophes.Replace(foldText(pair.b))
		if got := keyboardShape(x, y); got != pair.shape {
			t.Errorf("%q/%q: shape %v, want %v", pair.a, pair.b, got, pair.shape)
		}
		if keyboardPanelShape(x, y) {
			t.Errorf("%q/%q: shape %v is captured", pair.a, pair.b, pair.shape)
		}
		if base, score := keyboardBase(t, pair.a, pair.b); base || score != pair.score {
			t.Errorf("%q/%q: base %v %v, origin/main false %v", pair.a, pair.b, base, score, pair.score)
		}
		want, wantScore := wantLastNameFallback(t, x, y)
		if want != pair.match {
			t.Errorf("%q/%q: flat score gives %v, want %v", pair.a, pair.b, want, pair.match)
		}
		if got, actual := fuzzyLastName(pair.a, pair.b, false); got != want || actual != wantScore {
			t.Errorf("%q/%q: got %v %v, want %v %v", pair.a, pair.b, got, actual, want, wantScore)
		}
	}
}

// Every uncaptured shape follows the flat score: generated pairs sweep lengths
// to 17, substitutions to 9 and trailing counts to 10. Substitutions run
// back from the end of the shorter name so the match key survives.
func TestLastNameKeyboardUncapturedSweepUsesFlatScore(t *testing.T) {
	const tail = "ertuiopasdf"
	neighbor := strings.NewReplacer("a", "s", "b", "n", "c", "v", "d", "f", "e", "r", "f", "g", "g", "h", "h", "j", "i", "o", "k", "l",
		"l", "k", "m", "n", "n", "m", "o", "p", "p", "o", "r", "t", "s", "d", "t", "y", "u", "i", "v", "b", "w", "e", "y", "u", "z", "x", "j", "k", "q", "w", "x", "c")
	captured, checked, flat := map[[3]int]bool{}, map[[3]int]bool{}, 0
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
					want, score := wantLastNameFallback(t, x, y)
					if base, _ := lastNameKeyboard(x, y); want && !base {
						flat++
					}
					for _, order := range [][2]string{{x, y}, {y, x}} {
						if got, actual := fuzzyLastName(order[0], order[1], false); got != want || actual != score {
							t.Errorf("%q/%q: got %v %v, want %v %v", order[0], order[1], got, actual, want, score)
						}
					}
				}
			}
		}
	}
	t.Logf("%d captured and %d uncaptured shapes; the flat score adds %d uncaptured pairs", len(captured), len(checked), flat)
	if len(captured) < 55 || len(checked) < 340 || flat < 300 {
		t.Fatalf("sweep reached %d captured and %d uncaptured shapes, %d flat matches", len(captured), len(checked), flat)
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

// singleKeyboardSubstitution counts positions in runes, as keyboardSimilarity does.
func singleKeyboardSubstitution(a, b string) bool {
	different := 0
	for len(a) > 0 && len(b) > 0 {
		ra, na := utf8.DecodeRuneInString(a)
		rb, nb := utf8.DecodeRuneInString(b)
		if ra != rb {
			different++
			if different > 1 {
				return false
			}
		}
		a, b = a[na:], b[nb:]
	}
	return len(a) == 0 && len(b) == 0 && different == 1
}

// keyboardFallbackPair reports whether two names reach the keyboard
// fallback: not an exact match, same key, different Metaphone 3 code, more
// than a single substitution, no terminal swap, and no base keyboard match.
// It returns the prepared names.
func keyboardFallbackPair(a, b string) (string, string, bool) {
	x, y := foldText(a), foldText(b)
	if x == y || lastNameWordKey(x) != lastNameWordKey(y) {
		return "", "", false
	}
	x, y = lastNameApostrophes.Replace(x), lastNameApostrophes.Replace(y)
	if x == y || lastNameMetaphone3(x) == lastNameMetaphone3(y) || singleKeyboardSubstitution(x, y) {
		return "", "", false
	}
	if _, _, swap := lastNameTerminalSwap(x, y); swap {
		return "", "", false
	}
	if matched, _ := lastNameKeyboard(x, y); matched {
		return "", "", false
	}
	return x, y, true
}

// keyboardNativePanel is one panel of LastName values with the native
// confidence of each matched ordered pair.
type keyboardNativePanel struct {
	name   string
	values []string
	native map[[2]int]float64
}

// keyboardFitPanels are the native LastName panels the fallback was fitted
// to: the fuzzy_name_keys panels and the LastName rows of
// duplicate_fuzzy_matching.
func keyboardFitPanels(t *testing.T) []keyboardNativePanel {
	t.Helper()
	var out []keyboardNativePanel
	cases, observations := fuzzyNameKeysData(t)
	for name, panel := range cases {
		if panel.Kind == "panel" && panel.Item == "LastName" {
			out = append(out, keyboardNativePanel{name, panel.Values, fuzzyNameKeysPanel(t, observations[name], len(panel.Values))})
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
		// Null values keep their index so the native pairs line up.
		values := make([]string, len(row.Values))
		for i, value := range row.Values {
			if value != nil {
				values[i] = *value
			}
		}
		native := map[[2]int]float64{}
		for _, m := range row.Matches {
			native[[2]int{int(m[0]), int(m[1])}] = m[2]
		}
		out = append(out, keyboardNativePanel{cells[0] + "/" + cells[2], values, native})
	}
	return out
}

// The gate table is exactly the shapes of the fitted pairs that reach the
// fallback (keyboardFallbackPair) and that native matches in either
// direction. A shape whose captured pairs native never matches gives the
// fallback nothing to recover and keeps the base result.
func TestLastNameKeyboardPanelCells(t *testing.T) {
	var want [16][9]uint16
	for _, panel := range keyboardFitPanels(t) {
		for i, a := range panel.values {
			for j, b := range panel.values {
				if j <= i || a == "" || b == "" {
					continue
				}
				x, y, ok := keyboardFallbackPair(a, b)
				if !ok || panel.native[[2]int{i, j}] == 0 && panel.native[[2]int{j, i}] == 0 {
					continue
				}
				shape := keyboardShape(x, y)
				want[shape[0]][shape[1]] |= 1 << uint(shape[2])
			}
		}
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

// keyboardMarginPanels are every native LastName panel that reaches the
// fallback: the fitted panels, the keyboard and hkeyboard groups of
// duplicate_fuzzy_panels, and the blind twin, second and third blind panels
// (tk01-tk22, bk01 on and ck01 on; the ctl_ and rep_ controls replay earlier panels).
func keyboardMarginPanels(t *testing.T) []keyboardNativePanel {
	t.Helper()
	out := keyboardFitPanels(t)
	values, observations := fuzzyPanelsLastName(t, "keyboard", "hkeyboard")
	for _, name := range slices.Sorted(maps.Keys(values)) {
		out = append(out, keyboardNativePanel{name, values[name], fuzzyNameKeysPanel(t, observations[name], len(values[name]))})
	}
	return append(out, keyboardTwinPanels(t, "tkeyboard", "bkeyboard", "ckeyboard")...)
}

// keyboardTwinPanels reads the blind panels of
// internal/dml/testdata/conformance/duplicate_fuzzy_panels (four native
// observations each for the anonymous-only twin; eight for bk, both routes) without their ctl_ and rep_ controls.
func keyboardTwinPanels(t *testing.T, groups ...string) []keyboardNativePanel {
	t.Helper()
	var out []keyboardNativePanel
	for _, group := range groups {
		// tkeyboard and ckeyboard are anonymous only (4 rows a panel); bkeyboard has both routes (8).
		per := 4
		if group == "bkeyboard" {
			per = 8
		}
		values, observations := fuzzyPanelsLastNameFrom(t, fuzzyPanelsTwinDir, per, group)
		for _, name := range slices.Sorted(maps.Keys(values)) {
			if strings.HasPrefix(name, "ctl_") || strings.HasPrefix(name, "rep_") {
				continue
			}
			out = append(out, keyboardNativePanel{name, values[name], fuzzyNameKeysPanel(t, observations[name], len(values[name]))})
		}
	}
	return out
}

// The fallback margin clears the highest Keyboard Distance score of every
// captured native non-match (an ordered pair, in any shape) that reaches the
// fallback in any panel, by at least 0.5. The highest is "Ingleborn" /
// "Inglebornxbqm" (tk14, 92.2615); the k1-k3 and hk panels alone reach 90.84.
func TestLastNameKeyboardFallbackMargin(t *testing.T) {
	highest, pairs, top := 0.0, 0, ""
	for _, panel := range keyboardMarginPanels(t) {
		for i, a := range panel.values {
			for j, b := range panel.values {
				if i == j || a == "" || b == "" || panel.native[[2]int{i, j}] > 0 {
					continue
				}
				x, y, ok := keyboardFallbackPair(a, b)
				if !ok {
					continue
				}
				if score, ok := keyboardDistance(x, y); ok && score >= 90 {
					pairs++
					if score > highest {
						highest, top = score, panel.name+" "+a+"/"+b
					}
				}
			}
		}
	}
	t.Logf("%d captured non-matches at 90 or more; highest %.4f (%s)", pairs, highest, top)
	if highest < 92.26 || highest > 92.27 {
		t.Errorf("highest captured non-match %.4f (%s), want 92.2615 until a new capture raises it", highest, top)
	}
	if keyboardFallbackMargin < highest+0.5 {
		t.Errorf("keyboardFallbackMargin %v is not 0.5 above the highest captured non-match %.4f", keyboardFallbackMargin, highest)
	}
}

// The flat score keeps the same margin: no captured native non-match that
// reaches the fallback scores keyboardFallbackMargin or more, in any shape.
func TestLastNameKeyboardFlatMargin(t *testing.T) {
	highest, pairs, top := 0.0, 0, ""
	for _, panel := range keyboardMarginPanels(t) {
		for i, a := range panel.values {
			for j, b := range panel.values {
				if i == j || a == "" || b == "" || panel.native[[2]int{i, j}] > 0 {
					continue
				}
				x, y, ok := keyboardFallbackPair(a, b)
				if !ok {
					continue
				}
				if score, ok := keyboardFlatDistance(x, y); ok && score >= 90 {
					pairs++
					if score > highest {
						highest, top = score, panel.name+" "+a+"/"+b
					}
				}
			}
		}
	}
	t.Logf("%d captured non-matches at 90 or more; highest %.4f (%s)", pairs, highest, top)
	if keyboardFallbackMargin <= highest || highest < 92.80 || highest > 92.82 {
		t.Errorf("keyboardFallbackMargin %v, highest captured flat non-match %.4f (%s), want 92.8091 below the margin until a new capture raises it", keyboardFallbackMargin, highest, top)
	}
}

// keyboardFlatDistance scores the spec's constants: one key step costs 0.0759
// and one trailing letter 0.2437, per unit of the longer length.
func TestKeyboardFlatDistanceValues(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want float64
		ok   bool
	}{
		{"riim", "riimn", 100 - 100*0.2437/5, true},
		{"ingleborn", "inglebornxbqm", 100 - 100*(0.2437*4)/13, true},
		{"abc", "abc", 100, true},
		{"qa", "za", 100 - 100*0.0759*2/2, true}, // q to z: top to bottom row is two steps
		{"qa", "qz", 100, true},                  // a to z: one row apart costs nothing
		{"qa", "qp", 100 - 100*0.0759*9/2, true},
		{"qa", "qs", 100 - 100*0.0759*1/2, true},
		{"harlow", "harløw", 0, false},
		{"harlow 2", "harlow 3", 0, false},
	} {
		got, ok := keyboardFlatDistance(c.a, c.b)
		if ok != c.ok || ok && math.Abs(got-c.want) > 1e-9 {
			t.Errorf("%q/%q: %v %v, want %v %v", c.a, c.b, got, ok, c.want, c.ok)
		}
	}
}

func TestKeyboardPanelShapeBoundaries(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"abc", "abxd", true},                                    // (4, 1, 1)
		{"ééé", "ééab", true},                                    // (4, 1, 1) in runes
		{"abcd", "abxy", false},                                  // (4, 2, 0): reaches the fallback, but native matches no such pair
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

const fuzzyKeyboardNeighborsDir = "testdata/conformance/fuzzy_keyboard_neighbors/"

// NATIVE fuzzy_keyboard_neighbors/org.tsv:2-17 (kn1, kn2): a custom LastName
// rule at API 62.0 and 67.0, anonymous and @IsTest, two repeats. These rows
// were captured after the fit and are not part of the gate table
// (TestLastNameKeyboardPanelCells reads fuzzy_name_keys and
// duplicate_fuzzy_matching only). kn1 holds neighbors of the captured
// five-substitution pair: "Farnsdelwicka" finds "Farnsdraeovka" natively at
// 97, shape (13, 5, 0), fitted score 92.0, below keyboardFallbackMargin, so
// Glade keeps the base result. kn2 holds two and three substitutions on other
// key pairs. Glade reports no pair native does not. The native pairs Glade
// lacks are either shape (13, 5, 1), which no fitted row holds, or gated
// pairs scoring 90 or more below the margin (keyboardMarginMiss).
func TestLastNameKeyboardNeighborsNativeRows(t *testing.T) {
	cases, observations := fuzzyNameKeysDataFrom(t, fuzzyKeyboardNeighborsDir)
	want := map[string]fuzzyNameKeysCount{
		"kn1": {native: 70, hit: 38, miss: 32, confidence: 2},
		"kn2": {native: 52, hit: 52, confidence: 2},
	}
	if len(cases) != len(want) {
		t.Fatalf("cases %d", len(cases))
	}
	for name, total := range want {
		c := cases[name]
		if c.Kind != "panel" || c.Rule != "N1Last" || c.Item != "LastName" {
			t.Fatalf("%s: case %+v", name, c)
		}
		native := fuzzyNameKeysPanel(t, observations[name], len(c.Values))
		var counts fuzzyNameKeysCount
		for i, a := range c.Values {
			for j, b := range c.Values {
				n := native[[2]int{i, j}]
				matched, confidence := fuzzyLastName(a, b, false)
				g := 0.0
				if matched {
					g = math.Floor(confidence + 0.5)
				}
				switch {
				case n == 0 && g == 0:
				case n > 0 && g > 0:
					counts.native++
					counts.hit++
					if n != g {
						counts.confidence++
						if math.Abs(n-g) > 1 {
							t.Errorf("%s/%s=>%s: native=%v glade=%v", name, a, b, n, g)
						}
					}
				case n > 0:
					counts.native++
					counts.miss++
					x, y := lastNameApostrophes.Replace(foldText(a)), lastNameApostrophes.Replace(foldText(b))
					if keyboardMarginMiss(a, b) {
						break
					}
					if shape := keyboardShape(x, y); shape != [3]int{13, 5, 1} || keyboardPanelShape(x, y) {
						t.Errorf("%s/%s=>%s: native=%v, Glade has no match at shape %v", name, a, b, n, shape)
					}
				default:
					counts.extra++
					t.Errorf("%s/%s=>%s: glade=%v, native has no match", name, a, b, g)
				}
			}
		}
		t.Logf("PANEL %s: %s", name, counts)
		if counts != total {
			t.Errorf("%s counts %s, want %s", name, counts, total)
		}
	}
	// The reviewed neighbor: origin/main ad1577dbc6 keeps it apart, native matches it.
	if matched, _ := keyboardBase(t, "Farnsdelwicka", "Farnsdraeovka"); matched {
		t.Error("the base scorer matches Farnsdelwicka/Farnsdraeovka")
	}
	if matched, confidence := fuzzyLastName("Farnsdelwicka", "Farnsdraeovka", false); matched {
		t.Errorf("Farnsdelwicka/Farnsdraeovka: matched at %v; its fitted score is below the fallback margin (native 97)", confidence)
	}
}
