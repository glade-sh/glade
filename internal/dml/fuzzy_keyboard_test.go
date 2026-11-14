package dml

import (
	"math"
	"testing"
)

// NATIVE fuzzy_name_keys/org.tsv (suf1, rsp1, ks1) and
// duplicate_fuzzy_matching/org.tsv (P4.LN.KB1, P4.LN.KB2): Keyboard Distance
// has no limit on substitutions or trailing characters, and the final
// position is cheaper.
func TestKeyboardDistanceNativeShapes(t *testing.T) {
	for _, c := range []struct {
		a, b       string
		match      bool
		confidence float64
	}{
		{"Harlow II", "Harlow IV", true, 97},         // suf1: one substitution, final position
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
