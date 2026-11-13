package dml

import (
	"math"
	"strings"
)

// A standard Contact can match J or J. with a one-word first name beginning
// with J. Two-letter prefixes and unrelated initials do not establish a match.
func standardFirstInitial(a, b string) bool {
	// This fallback accepts only ASCII, so the byte limit also bounds runes.
	if len(a) > maxFuzzyInputRunes || len(b) > maxFuzzyInputRunes || len(a) > 2 && len(b) > 2 {
		return false
	}
	if len(a) > len(b) || len(a) == len(b) && strings.HasSuffix(b, ".") {
		a, b = b, a
	}
	a = strings.TrimSuffix(a, ".")
	return len(a) == 1 && len(b) >= 2 && standardNameWord(a) && standardNameWord(b) && strings.EqualFold(a, b[:1])
}

func standardNameWord(s string) bool {
	for i := range len(s) {
		c := s[i]
		if !(c >= 'A' && c <= 'Z' || c >= 'a' && c <= 'z') {
			return false
		}
	}
	return true
}

func standardSurnameLengthsMayMatch(a, b string) bool {
	// The extra corroborated comparisons have native positives at lengths
	// 2 through 8. The inherited last-name item handles all other lengths.
	if len(a) != len(b) {
		short, long := min(len(a), len(b)), max(len(a), len(b))
		return (short == 3 || short == 4) && long == short+1
	}
	return len(a) >= 2 && len(a) <= 8
}

// With an exact first name and email the standard rule can also match local
// edits to a surname whose phonetic key differs. Preserve the keyboard
// threshold for neighboring-letter swaps. A phone alone does not qualify.
func standardCorroboratedSurname(a, b string) (bool, float64) {
	if !standardSurnameLengthsMayMatch(a, b) {
		return false, 0
	}
	if !standardNameWord(a) || !standardNameWord(b) {
		return false, 0
	}
	a, b = strings.ToLower(a), strings.ToLower(b)
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(a) != len(b) {
		if a != b[:len(a)] {
			return false, 0
		}
		score, ok := keyboardSimilarity(a, b)
		return ok && score >= 90, standardTrailingLetterConfidence(a, averageWith100(score))
	}
	// LocalEditsBehavior finds a five-letter surname when only its final nasal
	// consonant changes between the neighboring m and n keys, in either order.
	if len(a) == 5 && a[:4] == b[:4] && (a[4] == 'm' && b[4] == 'n' || a[4] == 'n' && b[4] == 'm') {
		return true, 100
	}
	// All new two-character swaps are terminal. The single three-character
	// rotation in the native name panel is its five-letter terminal control.
	i := len(a) - 2
	if a[:i] != b[:i] {
		if len(a) != 5 || a[:i-1] != b[:i-1] {
			return false, 0
		}
		i--
	}
	left, right := a[i:], b[i:]
	if left == right {
		return false, 0
	}
	if len(left) == 2 {
		if left[0] != right[1] || left[1] != right[0] {
			return false, 0
		}
	} else if left[1:]+left[:1] != right && left[2:]+left[:2] != right {
		return false, 0
	}
	if lastNameMetaphone3Equal(a, b) {
		return true, 100
	}
	cost := 0.0
	for i := range len(left) {
		cost += keyboardCost(rune(left[i]), rune(right[i]))
	}
	score := 100 * (1 - cost/float64(len(a)))
	confidence := averageWith100(score)
	if score >= 92 {
		if len(left) == 2 {
			return standardTerminalSwap(a, confidence)
		}
		confidence = standardLocalEditConfidence(a, left, right)
	}
	return score >= 92, confidence
}

// These rounded grades are fitted to the local-edit panels. They use the
// whole-name key positions to distinguish the same edit at different stems.
func standardLocalEditConfidence(name, left, right string) float64 {
	steps := 0
	for i := range len(left) {
		if left[i] != right[i] {
			a, b := qwertyPosition[rune(left[i])], qwertyPosition[rune(right[i])]
			steps += max(1, abs(a[1]-b[1]))
		}
	}
	return math.Round(100 - float64(60*steps)/float64(16*len(name)+standardKeyboardPositionSum(name)))
}

func standardTrailingLetterConfidence(short string, ordinary float64) float64 {
	if len(short) != 4 {
		return ordinary
	}
	return math.Round(100 - 80/float64(4*len(short)+standardKeyboardPositionSum(short)))
}

// KeyboardBehavior, PrefixBehavior and HoldoutBehavior in duplicate_save_behavior
// measure nonphonetic terminal swaps with an equal first name and email. At a
// fixed key separation and length, eligibility and rounded confidence vary
// with the positions of the unchanged letters too. The captured groups split
// by the sum of their Manhattan distances to the midpoint between Y and U.
// Use doubled coordinates so the half-column origin and cutoffs are exact.
func standardTerminalSwap(name string, ordinary float64) (bool, float64) {
	a := qwertyPosition[rune(name[len(name)-2])]
	b := qwertyPosition[rune(name[len(name)-1])]
	rows, columns := abs(a[0]-b[0]), abs(a[1]-b[1])
	switch len(name) {
	case 2:
		if columns > 1 || rows > 1 {
			return false, 0
		}
		if columns == 1 {
			if standardKeyboardPositionSum(name) >= 18 {
				return true, 98
			}
			return true, 97
		}
	case 3, 8:
		if rows <= 1 && (len(name) == 3 && columns == 1 || len(name) == 8 && columns == 5) {
			left := name[len(name)-2:]
			return true, standardLocalEditConfidence(name, left, left[1:]+left[:1])
		}
	case 6:
		if columns == 4 && rows <= 1 {
			if standardKeyboardPositionSum(name) < 44 {
				return false, 0
			}
			return true, 97
		}
	}
	return true, ordinary
}

func standardKeyboardPositionSum(name string) int {
	total := 0
	for _, c := range name {
		key := qwertyPosition[c]
		total += abs(2*key[1]-11) + 2*key[0]
	}
	return total
}

// A same-key pair may already have qualified through the standard last-name
// item's keyboard branch. Apply the same corroborated swap verdict there.
func standardIsTerminalSwap(a, b string) bool {
	if (len(a) != 2 && len(a) != 3 && len(a) != 6 && len(a) != 8) || len(a) != len(b) || !standardNameWord(a) || !standardNameWord(b) {
		return false
	}
	a, b = strings.ToLower(a), strings.ToLower(b)
	i := len(a) - 2
	return a[:i] == b[:i] && a[i] != a[i+1] && a[i] == b[i+1] && a[i+1] == b[i]
}
