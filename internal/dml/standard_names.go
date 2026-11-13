package dml

import "strings"

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
	return len(a) >= 2 && len(b) >= 2 && len(a) <= maxFuzzyInputRunes && len(b) <= maxFuzzyInputRunes && abs(len(a)-len(b)) <= 1
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
		return ok && score >= 90, averageWith100(score)
	}
	start := 0
	for start < len(a) && a[start] == b[start] {
		start++
	}
	end := len(a)
	for end > start && a[end-1] == b[end-1] {
		end--
	}
	left, right := a[start:end], b[start:end]
	switch len(left) {
	case 2:
		if left[0] != right[1] || left[1] != right[0] {
			return false, 0
		}
	case 3:
		if left[1:]+left[:1] != right && left[2:]+left[:2] != right {
			return false, 0
		}
	default:
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
	if len(left) == 2 && end == len(a) && score >= 92 {
		return standardTerminalSwap(a, confidence)
	}
	return score >= 92, confidence
}

// The short keyboard panels in duplicate_save_behavior enumerate adjacent
// horizontal keys and one-column diagonals. Nonphonetic reversed two-letter
// names match only near keys. The lower-left part of each keyboard row has
// a one-point smaller penalty, including exchanges across neighboring rows.
func standardTerminalSwap(name string, ordinary float64) (bool, float64) {
	a := qwertyPosition[rune(name[len(name)-2])]
	b := qwertyPosition[rune(name[len(name)-1])]
	rowDelta, colDelta := a[0]-b[0], a[1]-b[1]
	columns := abs(colDelta)
	rightDiagonal := abs(rowDelta) == 1 && rowDelta*colDelta > 0
	switch {
	case len(name) == 2:
		if columns > 1 || abs(rowDelta) > 1 {
			return false, 0
		}
		if columns == 1 && (rowDelta == 0 || rightDiagonal) {
			if min(a[1], b[1]) <= max(a[0], b[0]) {
				return true, 98
			}
			return true, 97
		}
	case len(name) == 6 && columns == 4 && rightDiagonal:
		return true, 97
	}
	return true, ordinary
}

// A same-key pair may already have qualified through the standard last-name
// item's keyboard branch. Apply the same corroborated swap verdict there.
func standardIsTerminalSwap(a, b string) bool {
	if (len(a) != 2 && len(a) != 6) || len(a) != len(b) || !standardNameWord(a) || !standardNameWord(b) {
		return false
	}
	a, b = strings.ToLower(a), strings.ToLower(b)
	i := len(a) - 2
	return a[:i] == b[:i] && a[i] != a[i+1] && a[i] == b[i+1] && a[i+1] == b[i]
}
