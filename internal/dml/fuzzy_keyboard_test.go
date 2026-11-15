package dml

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// NATIVE fuzzy_name_keys/org.tsv (suf1, rsp1, ks1) and
// duplicate_fuzzy_matching/org.tsv (P4.LN.KB1, P4.LN.KB2): Keyboard Distance
// uses positional substitutions and trailing characters; the final position
// is cheaper. Its caller limits the fallback to captured panel ranges.
func TestKeyboardDistanceNativeShapes(t *testing.T) {
	for _, c := range []struct {
		a, b       string
		match      bool
		confidence float64
	}{
		{"Harlow II", "Harlow IV", true, 98},         // suf1: native 97; preserve the base score
		{"Harlow Jr", "Harlowe", true, 97},           // suf1: a space against a letter, two trailing; native 96 (fuzzyNameKeysResiduals)
		{"Farnsdelwick", "Farnsdraeovk", true, 96},   // ks1: six substitutions
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
		t.Error("standard Last Name matched six substitutions")
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

// Outside the observed panel envelope, the custom fallback keeps the base
// keyboard result, including its confidence for nonmatches.
func TestLastNameKeyboardOutsidePanelBounds(t *testing.T) {
	for _, pair := range [][2]string{
		{"Farnsdelwickaa", "Farnsdraeovkaa"},       // longer than any eligible panel pair
		{"Farnsdelwick", "Farnsdraeovkzzzzzzzzzz"}, // trailing difference above panel range
		{"erkecpmeitrn", "erkqudnukxik"},           // substitutions above panel range
		{"ipyqh", "wuabq"},                         // shorter than the eligible panel range
	} {
		x, y := foldText(pair[0]), foldText(pair[1])
		if lastNameWordKey(x) != lastNameWordKey(y) || lastNameMetaphone3(x) == lastNameMetaphone3(y) {
			t.Errorf("%q/%q does not reach keyboard scoring", pair[0], pair[1])
			continue
		}
		want, score := lastNameKeyboard(x, y)
		got, actual := fuzzyLastName(pair[0], pair[1], false)
		if got != want || actual != score {
			t.Errorf("%q/%q: got %v %v, base %v %v", pair[0], pair[1], got, actual, want, score)
		}
	}
}

func TestLastNameKeyboardPanelBounds(t *testing.T) {
	cases, _ := fuzzyNameKeysData(t)
	minima, maxima := [3]int{1 << 30, 1 << 30, 1 << 30}, [3]int{}
	for _, panel := range cases {
		if panel.Kind != "panel" || panel.Item != "LastName" {
			continue
		}
		for i, a := range panel.Values {
			for _, b := range panel.Values[i+1:] {
				x, y := foldText(a), foldText(b)
				if x == y || lastNameWordKey(x) != lastNameWordKey(y) {
					continue
				}
				x, y = lastNameApostrophes.Replace(x), lastNameApostrophes.Replace(y)
				if x == y || lastNameMetaphone3(x) == lastNameMetaphone3(y) {
					continue
				}
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
				for j, value := range [3]int{diff, len(ry) - len(rx), len(ry)} {
					if value < minima[j] {
						minima[j] = value
					}
					if value > maxima[j] {
						maxima[j] = value
					}
				}
			}
		}
	}
	if minima != [3]int{0, 0, 6} || maxima != [3]int{8, 9, 13} {
		t.Fatalf("eligible panel bounds changed: min %v max %v", minima, maxima)
	}
}
