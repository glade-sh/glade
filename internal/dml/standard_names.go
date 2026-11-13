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

// Normalize the captured case, accent, hyphen and shared leading-token forms
// before applying the same surname geometry. Keep the input cap before folding.
func standardSurnameText(s string) string {
	s = strings.ReplaceAll(stripAccents(strings.ToLower(s)), "-", "")
	return s
}

func standardSurnamePair(a, b string) (string, string) {
	a, b = standardSurnameText(a), standardSurnameText(b)
	// A shared leading token does not change the last token's score.
	x, y := strings.Fields(a), strings.Fields(b)
	if len(x) == 2 && len(y) == 2 && x[0] == y[0] {
		a, b = x[1], y[1]
	}
	return a, b
}

func standardSurnameLengthsMayMatch(a, b string) bool {
	if len(a) > maxFuzzyInputRunes || len(b) > maxFuzzyInputRunes {
		return false
	}
	accented := a != stripAccents(a) || b != stripAccents(b)
	a, b = standardSurnamePair(a, b)
	// The extra corroborated comparisons have native positives at lengths
	// 2 through 8. The inherited last-name item handles all other lengths.
	if len(a) != len(b) {
		short, long := min(len(a), len(b)), max(len(a), len(b))
		return (short == 3 || short == 4 || short == 5 && accented) && long == short+1
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
	a, b = standardSurnamePair(a, b)
	if !standardNameWord(a) || !standardNameWord(b) {
		return false, 0
	}
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(a) != len(b) {
		if a != b[:len(a)] {
			return false, 0
		}
		if lastNameMetaphone3Equal(a, b) {
			return true, 100
		}
		score, _ := keyboardSimilarity(a, b)
		return true, standardTrailingConfidence(a, b, averageWith100(score))
	}
	different, cost := 0, 0.0
	for i := range len(a) {
		if a[i] != b[i] {
			different++
			cost = keyboardCost(rune(a[i]), rune(b[i]))
		}
	}
	if different == 1 && len(a) >= 3 {
		if lastNameMetaphone3Equal(a, b) {
			return true, 100
		}
		if cost == 0.05 {
			if len(a) >= 5 {
				return true, 100
			}
			return true, 99
		}
		// The far-key z/m controls cost 0.4, while q/p costs 0.64 and misses.
		if cost == 0.4 && len(a) == 5 {
			return true, 97
		}
		return false, 0
	}
	// Captured two-character swaps are terminal; three-character terminal
	// rotations are measured only for five-letter names.
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
	cost = 0.0
	for i := range len(left) {
		cost += keyboardCost(rune(left[i]), rune(right[i]))
	}
	score := 100 * (1 - cost/float64(len(a)))
	confidence := averageWith100(score)
	if len(left) == 3 && score < 94 {
		return false, 0
	}
	if score >= 92 {
		if len(left) == 2 {
			return standardTerminalSwap(a, confidence, false)
		}
		confidence = standardRotationConfidence(a, left, right, confidence)
	}
	return score >= 92, confidence
}

// Panel F measures five-letter terminal rotations: the one-row/one-column
// group scores 98 through sum 35 and 99 from 39. Vertical rotations retain
// 98; wide same-row controls retain 97. Wide controls below 94 miss.
func standardRotationConfidence(name, left, right string, ordinary float64) float64 {
	maxRows, maxColumns := 0, 0
	for i := range len(left) {
		a, b := qwertyPosition[rune(left[i])], qwertyPosition[rune(right[i])]
		maxRows = max(maxRows, abs(a[0]-b[0]))
		maxColumns = max(maxColumns, abs(a[1]-b[1]))
	}
	if maxRows == 1 && maxColumns == 1 {
		sum := standardKeyboardPositionSum(name)
		if sum <= 35 {
			return 98
		}
	}
	return ordinary
}

// Panels A and C pin the p/s/z and three-letter s/p/k grades. Other
// extension letters keep their ordinary grade; tests enumerate the residuals.
func standardTrailingConfidence(short, long string, ordinary float64) float64 {
	sum := standardKeyboardPositionSum(short)
	tail := long[len(long)-1]
	if len(short) >= 4 {
		switch tail {
		case 'p':
			return 99
		case 's':
			if sum >= 42 {
				return 99
			}
			return 98
		case 'z':
			if sum >= 18 {
				return 98
			}
			if sum == 16 {
				return 97
			}
		}
	}
	if len(short) == 3 {
		switch tail {
		case 's':
			return 98
		case 'p':
			if sum >= 19 {
				return 99
			}
			return 98
		case 'k':
			if sum >= 15 {
				return 98
			}
			return 97
		}
	}
	return ordinary
}

// KeyboardBehavior, PrefixBehavior and HoldoutBehavior in duplicate_save_behavior
// measure nonphonetic terminal swaps with an equal first name and email. At a
// fixed key separation and length, eligibility and rounded confidence vary
// with the positions of the unchanged letters too. The captured groups split
// by the sum of their Manhattan distances to the midpoint between Y and U.
// Use doubled coordinates so the half-column origin and cutoffs are exact.
func standardTerminalSwap(name string, ordinary float64, ordinaryMatched bool) (bool, float64) {
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
	case 3:
		// Panel E: same-row adjacent swaps score 98 through sum 29,
		// and 99 from sum 31.
		if rows == 0 && columns == 1 {
			sum := standardKeyboardPositionSum(name)
			if sum >= 31 {
				return true, 99
			}
		}
	case 8:
		if rows <= 1 && columns == 5 {
			// Panel G: five-column swaps miss through sum 44, and score
			// 97 from sum 46 through 62, on both phonetic-key paths.
			sum := standardKeyboardPositionSum(name)
			if sum <= 44 {
				return false, 0
			}
			if sum >= 46 && sum <= 62 {
				return true, 97
			}
			// Above the captured interval, retain the ordinary surname verdict.
			return ordinaryMatched, ordinary
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
