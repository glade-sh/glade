package dml

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// firstNameInitialKeyed is the First Name method behind the key
// First_Name (1,1): the initial of the first word, punctuation removed,
// selects the pair instead of the method's phonetic key. Equal text and a
// Name Variant entry still score 100 first (the key substitutes aliases
// before it takes the initial), and the pairs the tables keep apart stay
// apart.
func firstNameInitialKeyed(a, b string) (bool, float64) {
	return newFirstNameInitialSide(a).keyed(b)
}

// firstNameInitialSide is the candidate side of firstNameInitialKeyed, read
// once when one name is compared with many.
type firstNameInitialSide struct {
	x, fx string
	ix    rune
}

func newFirstNameInitialSide(a string) firstNameInitialSide {
	x := strings.ToLower(capFuzzyInput(strings.TrimSpace(a)))
	fx := foldLowered(x)
	return firstNameInitialSide{x: x, fx: fx, ix: firstNameInitialOf(fx)}
}

// keyed is firstNameInitialKeyed(a, b) for the side's name a.
func (side firstNameInitialSide) keyed(b string) (bool, float64) {
	y := strings.ToLower(capFuzzyInput(strings.TrimSpace(b)))
	fy := foldLowered(y)
	if side.x == y || nameVariantOf(side.fx, fy) {
		return true, 100
	}
	if iy := firstNameInitialOf(fy); side.ix == utf8.RuneError || side.ix != iy || nameKeptApart[orderedPair(side.fx, fy)] {
		return false, 0
	}
	return firstNameAlgorithms(side.x, y, side.fx, fy)
}

// firstNameInitialOf is the first letter or digit of a folded first name's
// first word, or utf8.RuneError when it has none.
func firstNameInitialOf(folded string) rune {
	word := firstNameWord(folded)
	if word == "" {
		return utf8.RuneError
	}
	r, _ := utf8.DecodeRuneInString(word)
	return r
}

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
	// The four accented five-to-six pairs also fit dropping the accented
	// letter. Keep the conservative accent gate; that mechanism is unproven.
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
		if lastNameMetaphone3(a) == lastNameMetaphone3(b) {
			return true, 100
		}
		score, _ := keyboardSimilarity(a, b)
		return true, standardTrailingConfidence(a, b, averageWith100(score))
	}
	different, changed, cost := 0, 0, 0.0
	for i := range len(a) {
		if a[i] != b[i] {
			different++
			changed = i
			cost = keyboardCost(rune(a[i]), rune(b[i]))
		}
	}
	if different == 1 && len(a) >= 3 {
		if lastNameMetaphone3(a) == lastNameMetaphone3(b) {
			return true, 100
		}
		left, right := qwertyPosition[rune(a[changed])], qwertyPosition[rune(b[changed])]
		sum := min(standardKeyboardPositionSum(a), standardKeyboardPositionSum(b))
		// Panel J measures first, middle and final positions, both directions,
		// with same-row, vertical and diagonal neighbors at lengths 4 to 8.
		// The earlier panel D also measures three-letter terminal neighbors.
		if cost == 0.05 {
			if len(a) == 3 && (changed != len(a)-1 || left[0] != right[0]) {
				return false, 0
			}
			if len(a) >= 5 || len(a) == 4 && left[0] != right[0] && sum >= 34 {
				return true, 100
			}
			return true, 99
		}
		// Panels D and J's far-key rules use only the captured geometry,
		// never every equal-cost pair. Use the smaller position sum so both
		// directions reach the same boundary.
		if left[0] == 2 && right[0] == 2 && abs(left[1]-right[1]) == 6 {
			switch len(a) {
			case 4:
				if sum >= 40 {
					return true, 97
				}
			case 5:
				return true, 97
			case 6:
				if sum >= 50 {
					return true, 98
				}
				return true, 97
			case 7, 8:
				return true, 98
			}
		}
		if left[0] == 0 && right[0] == 0 && abs(left[1]-right[1]) == 9 && (len(a) == 8 || len(a) == 7 && sum >= 51) {
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
	if lastNameMetaphone3(a) == lastNameMetaphone3(b) {
		return true, 100
	}
	cost = 0.0
	for i := range len(left) {
		cost += keyboardCost(rune(left[i]), rune(right[i]))
	}
	score := float64(100 * (1 - cost/float64(len(a))))
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

// standardFirstNameJaroWinkler is the standard First Name threshold, the
// documented 85: on the email path "John" finds "Jo" (0.867) and "Jo" finds
// "Jose" (0.867); "Jo" never finds "Johny" (0.84) nor "Jon" "Jose" (0.778)
// (the standard fixture's Field.Contact.First and NameConfidence rows).
const standardFirstNameJaroWinkler = 0.85

// standardFirstNameScore is the standard First Name item without a match
// key: Exact, Initials, Jaro-Winkler, Metaphone 3 and Name Variant, the best
// of them (Salesforce Help lists Metaphone 3 for the standard rules only).
// Names that differ only in spaces and punctuation are equal ("Mary Ann",
// "MaryAnn" and "Mary-Ann"; B2 s_cmp_c), and equal Metaphone 3 codes score
// 100 ("Jane" finds "John", "Maira" finds "Mary"; Field.Contact.First). The
// score is the algorithm's, 100 or the Jaro-Winkler value as a percentage.
func standardFirstNameScore(a, b string) (bool, float64) {
	x, y := strings.ToLower(strings.TrimSpace(capFuzzyInput(a))), strings.ToLower(strings.TrimSpace(capFuzzyInput(b)))
	fx, fy := foldLowered(x), foldLowered(y)
	if x == y || nameVariantOf(fx, fy) || standardNameVariants[orderedPair(fx, fy)] || standardInitials(a, b, fx, fy) {
		return true, 100
	}
	jx, jy := standardNameLetters(fx), standardNameLetters(fy)
	if jx == jy && jx != "" {
		return true, 100
	}
	if code := lastNameMetaphone3(jx); code != "" && code == lastNameMetaphone3(jy) {
		return true, 100
	}
	score := jaroWinkler(x, y)
	return score >= standardFirstNameJaroWinkler-1e-9, 100 * score
}

// standardNameLetters is a folded name with everything but letters and
// digits removed.
func standardNameLetters(folded string) string {
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, folded)
}

// standardInitialKeyedFirst is the first-name share of the phone path,
// whose key is First_Name (1,1) Last_Name Phone. Both names must be present.
// The standard First Name verdict stands, and Initials and names that differ
// only in spaces and punctuation also pass when the initials agree (B2 s_in_c
// and s_ctl_c_b over phone: "Y" finds "Y R S", "YR" finds "Y R", "Mary Ann"
// finds "MaryAnn"). The confidence uses the standard first-name scale.
func standardInitialKeyedFirst(a, b string) (float64, bool) {
	a, b = strings.TrimSpace(a), strings.TrimSpace(b)
	if a == "" || b == "" {
		return 0, false
	}
	a, b = capFuzzyInput(a), capFuzzyInput(b)
	fx, fy := foldText(a), foldText(b)
	if ix := firstNameInitialOf(fx); ix != utf8.RuneError && ix == firstNameInitialOf(fy) {
		if standardInitials(a, b, fx, fy) || standardNameLetters(fx) == standardNameLetters(fy) {
			return 100, true
		}
	}
	if matched, confidence := standardFirstName(a, b); matched {
		// standardFirstName reports (100 + score) / 2.
		return standardFirstNameConfidence(2*confidence - 100), true
	}
	return 0, false
}

// standardSurnamePanelShape reports a surname pair inside the measured
// exact-first-name keyboard panels (standardCorroboratedSurname): single
// words of letters with the lengths those panels cover. Their misses are
// native verdicts too.
func standardSurnamePanelShape(a, b string) bool {
	if !standardSurnameLengthsMayMatch(a, b) {
		return false
	}
	x, y := standardSurnamePair(a, b)
	return standardNameWord(x) && standardNameWord(y)
}

// standardInitials is the Initials algorithm on two first names (fx, fy their
// folds) whose typed initials are ASCII letters: natively an initial "K"
// never finds a name typed with the Kelvin sign, U+212A ("Karl"), which
// folding would turn into "karl" (duplicate_save_behavior calibration
// initials).
func standardInitials(a, b, fx, fy string) bool {
	return standardASCIIInitial(a) && standardASCIIInitial(b) && (firstNameInitials(fx, fy) || firstNameInitials(fy, fx))
}

// standardASCIIInitial reports a name whose first letter is ASCII.
func standardASCIIInitial(s string) bool {
	for _, r := range s {
		if unicode.IsLetter(r) {
			return r < utf8.RuneSelf
		}
	}
	return false
}

// standardNameVariants are first-name pairs the standard Contact rule scores
// 100 on the email path, in either direction, that the shared dictionary and
// Glade's Metaphone 3 codes do not give (NATIVE standard_first_names c9s1 and
// c9s2; duplicate_rule_match_keys B2 s_cmp 8; "Raymond" and "Rae", "Susan"
// and "Su" from the fuzzy_first_alias_keys standard panel i1sb). The table is
// pairwise, never closed transitively: natively "Edward" does not find
// "Theodore" although both find "Ted", nor "John" "Henry" although both find
// "Hank". No custom-rule row shows these pairs, so custom rules do not read it.
var standardNameVariants = map[[2]string]bool{
	orderedPair("alex", "xander"):    true,
	orderedPair("edward", "ted"):     true,
	orderedPair("hank", "jack"):      true,
	orderedPair("hank", "john"):      true,
	orderedPair("jack", "john"):      true,
	orderedPair("marjorie", "peggy"): true,
	orderedPair("mary", "molly"):     true,
	orderedPair("mary", "polly"):     true,
	orderedPair("matt", "matthew"):   true,
	orderedPair("molly", "polly"):    true,
	orderedPair("rae", "raymond"):    true,
	orderedPair("sally", "sarah"):    true,
	orderedPair("su", "susan"):       true,
}
