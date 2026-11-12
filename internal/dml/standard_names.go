package dml

import "strings"

// A standard Contact can match J or J. with a one-word first name beginning
// with J. Two-letter prefixes and unrelated initials do not establish a match.
func standardFirstInitial(a, b string) bool {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if len(a) > len(b) || len(a) == len(b) && strings.HasSuffix(b, ".") {
		a, b = b, a
	}
	a = strings.TrimSuffix(a, ".")
	if len(a) != 1 || len(b) < 2 || a[0] != b[0] {
		return false
	}
	for i := range len(b) {
		if b[i] < 'a' || b[i] > 'z' {
			return false
		}
	}
	return true
}

func standardSurnameLengthsMayMatch(a, b string) bool {
	return len(a) >= 2 && len(b) >= 2 && abs(len(a)-len(b)) <= 1
}

// With an exact first name and email the standard rule can also match local
// edits to a surname whose phonetic key differs. Preserve the keyboard
// threshold for neighboring-letter swaps. A phone alone does not qualify.
func standardCorroboratedSurname(a, b string) (bool, float64) {
	a, b = strings.ToLower(a), strings.ToLower(b)
	if len(a) > len(b) {
		a, b = b, a
	}
	if !standardSurnameLengthsMayMatch(a, b) {
		return false, 0
	}
	for i := range len(b) {
		if b[i] < 'a' || b[i] > 'z' || i < len(a) && (a[i] < 'a' || a[i] > 'z') {
			return false, 0
		}
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
	return score >= 92, averageWith100(score)
}
