package dml

import (
	"math"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"
)

// Fuzzy matching methods for custom matching rule items. The method names are
// the MatchingMethod picklist values of the metadata API. Each scorer follows
// the public description of the method (Salesforce Help, "Matching Methods
// Used in Matching Rules" and "Matching Algorithms Used with Matching
// Methods") and is fitted to native Datacloud.FindDuplicates verdicts and
// match confidences (the duplicate_fuzzy_matching conformance fixture; see
// docs/DUPLICATE_FUZZY_MATCHING.md for the recovered formulas). Where
// Salesforce uses an unpublished dictionary the tables hold only the pairs the
// native rows prove.
//
// Two mechanisms recur in every method: a match key that must agree before
// the pair is scored at all (the help article "How Match Keys Are Generated"),
// and a single-item rule confidence of (100 + algorithm score) / 2 for the
// "Maximum" methods; the "Weighted Average" methods (Phone, Street, ZIP)
// report their weighted score directly.

// FuzzyMethodSupported reports whether the matching method is one of the
// field-specific fuzzy methods this package scores.
func FuzzyMethodSupported(method string) bool {
	_, ok := fuzzyScorers[strings.ToLower(strings.TrimSpace(method))]
	return ok
}

// FuzzyMatch reports whether two non-blank field values match under the
// method. Callers handle blank values.
func FuzzyMatch(method, left, right string) bool {
	matched, _ := FuzzyScore(method, left, right, false)
	return matched
}

// FuzzyScore scores two values under the method. A blank (empty or
// whitespace-only) value on either side never matches; the rule path filters
// blanks before it gets here, so this only guards direct callers.
// Values longer than maxFuzzyInputRunes are truncated before scoring.
// The score is the
// match confidence a single-item rule reports for the pair (0 to 100); a
// matched pair always scores at least the method's threshold. matchBlanks is
// the item's blank handling, which the Street method applies to a blank
// street-number section.
func FuzzyScore(method, left, right string, matchBlanks bool) (bool, float64) {
	key := strings.ToLower(strings.TrimSpace(method))
	scorer, ok := fuzzyScorers[key]
	if !ok {
		return false, 0
	}
	if custom, ok := customFuzzyScorers[key]; ok {
		scorer = custom
	}
	left, right = strings.TrimSpace(left), strings.TrimSpace(right)
	if left == "" || right == "" {
		return false, 0
	}
	return scorer(capFuzzyInput(left), capFuzzyInput(right), matchBlanks)
}

// maxFuzzyInputRunes bounds the text each scorer compares. Jaro-Winkler,
// keyboard distance and edit distance are quadratic in length. Duplicate rules
// read field values, and no field a fuzzy method applies to holds more than
// 255 characters (Street and the name, company and title fields; Phone is
// 40), so a longer value cannot occur natively. The cap is four times the
// largest of those limits, so no real value is ever shortened. Only the first
// maxFuzzyInputRunes runes of a longer value are compared.
const maxFuzzyInputRunes = 1024

func capFuzzyInput(s string) string {
	if len(s) <= maxFuzzyInputRunes {
		return s
	}
	n := 0
	for i := range s {
		if n == maxFuzzyInputRunes {
			return s[:i]
		}
		n++
	}
	return s
}

type fuzzyScorer func(a, b string, matchBlanks bool) (bool, float64)

var fuzzyScorers = map[string]fuzzyScorer{
	"firstname":   fuzzyFirstName,
	"lastname":    fuzzyLastName,
	"companyname": fuzzyCompany,
	"phone":       fuzzyPhone,
	"city":        fuzzyCity,
	"street":      fuzzyStreet,
	"zip":         fuzzyZip,
	"title":       fuzzyTitle,
}

// ---- shared text helpers ----

func foldText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

// foldLowered is foldText of a lower-cased, trimmed text; it returns the
// text itself when its white space is already single ASCII spaces.
func foldLowered(s string) string {
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 0x80 || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r' || c == ' ' && (i+1 == len(s) || s[i+1] == ' ') {
			return strings.Join(strings.Fields(s), " ")
		}
	}
	return s
}

// stripAccents removes combining marks: "García" becomes "Garcia".
func stripAccents(s string) string {
	var out []rune
	for _, r := range norm.NFD.String(s) {
		if !unicode.Is(unicode.Mn, r) {
			out = append(out, r)
		}
	}
	return string(out)
}

func isASCIILetter(r rune) bool { return r >= 'a' && r <= 'z' }

func levenshtein(a, b []rune) int {
	if len(a) == 0 {
		return len(b)
	}
	if len(b) == 0 {
		return len(a)
	}
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// editSimilarity is the Edit Distance score: 100 * (1 - distance / longer
// length). The help example "VP Sales" against "VP of Sales" scores 73.
func editSimilarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	longest := max(len(ra), len(rb))
	if longest == 0 {
		return 100
	}
	return 100 * (1 - float64(levenshtein(ra, rb))/float64(longest))
}

// averageWith100 is the single-item rule confidence of the Maximum methods.
func averageWith100(score float64) float64 { return (100 + score) / 2 }

// ---- First Name: Exact, Initials, Jaro-Winkler, Name Variant; threshold 85 ----

// firstNameJaroWinkler is the lowest Jaro-Winkler value the rows match: "Daniel"
// against "Diane" (0.89) matches, "Ravi" against "Ravindraa" (0.8889) does not.
const firstNameJaroWinkler = 0.89

// fuzzyFirstName: a is the candidate, b the existing record. Equal text
// (letter case and edge spaces aside) and a Name Variant entry of the
// candidate score 100; the dictionary is directional ("Rosa" finds "Rose",
// "Rose" does not find "Rosa"). Otherwise the two names are compared only when
// their first words, punctuation removed, are equal or share a match key, the
// first two characters of the Double Metaphone code ("Vorkel" and "Volkel" are
// never compared, "Vorkel" and "Vorpel" are; "Y R" and "Y R S" share the key
// of "Y", "YR" and "Y R" do not). A dictionary name whose key comes from its
// alias and not from its own spelling ("Kate" is keyed as "Katherine", so it
// matches neither "Katie" nor "Katte") matches by Jaro-Winkler only under
// that key ("Ty" finds "Tyr" and not "Tye"), and an initial does not find it
// ("T" does not find "Ty"). Then the
// Initials rule scores 100 (firstNameInitials), and otherwise Jaro-Winkler on
// the lower-cased text with its inner white space as typed ("Mary Ann" finds
// "Mary  Ann" at 99) decides at the threshold, with confidence
// (100 + score) / 2.
func fuzzyFirstName(a, b string, _ bool) (bool, float64) {
	x, y := strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	fx, fy := foldLowered(x), foldLowered(y)
	if x == y || nameVariantOf(fx, fy) {
		return true, 100
	}
	if wx, wy := firstNameWord(fx), firstNameWord(fy); wx != wy {
		if kx, ky := customFirstNameKey(wx), customFirstNameKey(wy); kx == "" || kx != ky {
			return false, 0
		}
	}
	return firstNameAlgorithms(x, y, fx, fy)
}

// firstNameAlgorithms scores a pair of lower-cased first names (x, y) and
// their folds (fx, fy) once a match key has let the pair through: the
// Initials rule scores 100, otherwise Jaro-Winkler decides at the threshold.
// The key is the matching rule's concern (customRuleKeyModes).
func firstNameAlgorithms(x, y, fx, fy string) (bool, float64) {
	if firstNameInitials(fx, fy) || firstNameInitials(fy, fx) {
		return true, 100
	}
	score := jaroWinkler(x, y)
	return score >= firstNameJaroWinkler-1e-9, averageWith100(100 * score)
}

// firstNameKey is the match key of a first name; empty for a dictionary name
// whose alias key no row shows.
func firstNameKey(name string) string {
	if key, ok := nameAliasKeys[name]; ok {
		return key
	}
	return firstNameCodeCache.get(name, func(s string) string { return doubleMetaphone(s, firstNameKeyLength) })
}

// customFirstNameKey is firstNameKey for a custom rule, which also keys the
// names of nameAliasKeysCustom by their alias.
func customFirstNameKey(word string) string {
	if key, ok := nameAliasKeysCustom[word]; ok {
		return key
	}
	return firstNameKey(word)
}

// firstNameWord is the first word of a folded name with every character
// that is not a letter or a digit removed: "p.w." keys as "pw", "jean-luc"
// as "jeanluc", "y. r. s." as "y".
func firstNameWord(folded string) string {
	word := folded
	if i := strings.IndexByte(word, ' '); i >= 0 {
		word = word[:i]
	}
	clean := true
	for _, r := range word {
		if !unicode.IsLetter(r) && !unicode.IsDigit(r) {
			clean = false
			break
		}
	}
	if clean {
		return word
	}
	return strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, word)
}

// firstNameInitials is the Initials algorithm on two folded names: the first
// word of initials, punctuation removed, is one letter, and name is a single
// word that starts with that letter after any leading punctuation. "P W G"
// finds "PW", "P.W.", "P-W" and "P" at 100; "P W" finds "P W G" only by
// Jaro-Winkler, and "Yancey" finds "Yancey R" only by Jaro-Winkler. An
// initial alone also finds a name of several words ("T." finds "Tye Ann" at
// 100, "T Ann" finds it only by Jaro-Winkler).
func firstNameInitials(initials, name string) bool {
	word := firstNameWord(initials)
	letter, size := utf8.DecodeRuneInString(word)
	if size == 0 || size != len(word) || !unicode.IsLetter(letter) {
		return false
	}
	if strings.IndexByte(name, ' ') >= 0 && strings.IndexByte(initials, ' ') >= 0 {
		return false
	}
	for _, r := range name {
		if unicode.IsLetter(r) {
			return r == letter
		}
		if !unicode.IsPunct(r) {
			return false
		}
	}
	return false
}

func orderedPair(a, b string) [2]string {
	if a > b {
		a, b = b, a
	}
	return [2]string{a, b}
}

const firstNameKeyLength = 2

// ---- Last Name: Exact, Keyboard Distance, Metaphone 3; threshold 90 ----

const lastNameKeyLength = 4

// fuzzyLastName: the match key is the Double Metaphone code of the last word,
// four characters at most (lastNameWordKey: "Frost" and "Frosr" are never
// compared, "Martinez" and "Martinpz" are; "Test 1" and "Last 1" are, "Test 1"
// and "Test 2" are not). Two names with equal Metaphone 3 codes score 100,
// whatever separates their words ("Harlow" finds "Harlow III", "Al-Rashid"
// finds "Alrashid"). Otherwise Keyboard Distance decides at 90 with
// confidence (100 + score) / 2.
func fuzzyLastName(a, b string, _ bool) (bool, float64) {
	x, y := foldText(a), foldText(b)
	if x == y {
		return true, 100
	}
	if lastNameWordKey(x) != lastNameWordKey(y) {
		return false, 0
	}
	x, y = lastNameApostrophes.Replace(x), lastNameApostrophes.Replace(y)
	if x == y || lastNameMetaphone3(x) == lastNameMetaphone3(y) {
		return true, 100
	}
	length, substitutions, trailing := keyboardNameShape(x, y)
	if substitutions == 1 && trailing == 0 {
		// A single substitution keeps the base scorer.
		return lastNameKeyboard(x, y)
	}
	if matched, confidence, swap := lastNameTerminalSwap(x, y); swap {
		return matched, confidence
	}
	// lastNameKeyboard without its terminal swap step, which did not apply.
	base, ok := keyboardSimilarity(x, y)
	matched, confidence := ok && base >= 90, averageWith100(base)
	if matched || !keyboardPanelCell(length, substitutions, trailing) {
		return matched, confidence
	}
	if score, ok := keyboardDistance(x, y); ok && score >= keyboardFallbackMargin {
		return true, averageWith100(score)
	}
	return matched, confidence
}

// lastNameAlgorithms scores two folded last names without a match key:
// equal text apart from apostrophes and equal Metaphone 3 codes score 100,
// otherwise Keyboard Distance decides.
func lastNameAlgorithms(x, y string) (bool, float64) {
	x, y = lastNameApostrophes.Replace(x), lastNameApostrophes.Replace(y)
	if x == y || lastNameMetaphone3(x) == lastNameMetaphone3(y) {
		return true, 100
	}
	return lastNameKeyboard(x, y)
}

// keyboardPanelShape gates the fallback to the joint shapes the native rows
// hold. A shape is (length, substitutions, trailing): the longer folded,
// apostrophe-stripped name in runes, the positional differences, and the
// unpaired suffix (keyboardNameShape). A shape passes when a captured pair of that
// shape that reaches the fallback is a native match (keyboardPanelCells). Every
// other shape keeps the base keyboard result.
func keyboardPanelShape(a, b string) bool {
	return keyboardPanelCell(keyboardNameShape(a, b))
}

// keyboardNameShape is (length, substitutions, trailing) of two prepared names.
func keyboardNameShape(a, b string) (length, substitutions, trailing int) {
	na, nb := 0, 0
	for len(a) > 0 && len(b) > 0 {
		ra, sa := utf8.DecodeRuneInString(a)
		rb, sb := utf8.DecodeRuneInString(b)
		if ra != rb {
			substitutions++
		}
		na++
		nb++
		a, b = a[sa:], b[sb:]
	}
	na += utf8.RuneCountInString(a)
	nb += utf8.RuneCountInString(b)
	if nb > na {
		return nb, substitutions, nb - na
	}
	return na, substitutions, na - nb
}

func keyboardPanelCell(length, substitutions, trailing int) bool {
	if length >= len(keyboardPanelCells) || substitutions >= len(keyboardPanelCells[length]) {
		return false
	}
	return keyboardPanelCells[length][substitutions]>>uint(trailing)&1 == 1
}

// keyboardPanelCells[length][substitutions] has bit t set when a captured
// pair with t trailing characters reaches the fallback and native matches it:
// fuzzy_name_keys/org.tsv:10-25,122-169 (cases.json supplies the panel
// values) and the LastName rows of duplicate_fuzzy_matching/org.tsv.
// TestLastNameKeyboardPanelCells rebuilds it from the native rows.
var keyboardPanelCells = [16][9]uint16{
	4:  {0x0000, 0x0002, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000},
	5:  {0x0000, 0x0002, 0x0002, 0x0000, 0x0001, 0x0000, 0x0000, 0x0000, 0x0000},
	6:  {0x0000, 0x0006, 0x0002, 0x0003, 0x0001, 0x0000, 0x0000, 0x0000, 0x0000},
	7:  {0x0000, 0x0006, 0x0007, 0x0003, 0x0003, 0x0000, 0x0000, 0x0000, 0x0000},
	8:  {0x0008, 0x0006, 0x0006, 0x0001, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000},
	9:  {0x0008, 0x000e, 0x0006, 0x0007, 0x0001, 0x0000, 0x0000, 0x0000, 0x0000},
	10: {0x0018, 0x000e, 0x0002, 0x000b, 0x0003, 0x0000, 0x0000, 0x0000, 0x0000},
	11: {0x0018, 0x0006, 0x0006, 0x0006, 0x0004, 0x0000, 0x0000, 0x0000, 0x0000},
	12: {0x0018, 0x000e, 0x000a, 0x000b, 0x0001, 0x0001, 0x0001, 0x0000, 0x0000},
	13: {0x0038, 0x0000, 0x0000, 0x0001, 0x0001, 0x0001, 0x0001, 0x0000, 0x0000},
	14: {0x0030, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000},
	15: {0x0040, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000, 0x0000},
}

// keyboardFallbackMargin is the one score the fallback needs, in every gated
// shape. The fitted costs are not reliable between 88 and about 92.3 in any
// shape: native non-matches score as high as 90.84 in substitution shapes
// (duplicate_fuzzy_panels k1-k3) and 92.26 in extension-only shapes (the
// blind twin tk01-tk22, "Ingleborn"/"Inglebornxbqm"). The highest captured
// native non-match of any shape that reaches the fallback, in every panel
// (fitted, k1-k3, hk1, hk2, tk01-tk22), is 92.2615; the margin sits 0.74
// above it. A pair scoring below it keeps the base result. The fitted score
// is a model, so no margin can be proved; TestLastNameKeyboardFallbackMargin
// pins the highest captured non-match and requires the margin above it, and
// the second blind panel (bk01 on) was scored with this margin before it was
// read. Per-shape floors are gone: a floor existed only where an earlier
// panel happened to hold a counterexample, and a fresh panel found 108 more.
const keyboardFallbackMargin = 93.0

var lastNameApostrophes = strings.NewReplacer("'", "", "\u2019", "")

// lastNameKeyboard is the Keyboard Distance step of the standard rules' Last
// Name on two folded names whose apostrophes are removed. It keeps the costs
// the standard rows were fitted with (keyboardSimilarity); the custom Last
// Name method also retains this score for previously accepted pairs.
func lastNameKeyboard(x, y string) (bool, float64) {
	if matched, confidence, swap := lastNameTerminalSwap(x, y); swap {
		return matched, confidence
	}
	score, ok := keyboardSimilarity(x, y)
	return ok && score >= 90, averageWith100(score)
}

// lastNameTerminalSwap decides two eight-letter names whose last two letters
// are swapped, for the captured five-column geometry only.
func lastNameTerminalSwap(x, y string) (matched bool, confidence float64, swap bool) {
	if len(x) != 8 || !standardIsTerminalSwap(x, y) {
		return false, 0, false
	}
	a, b := qwertyPosition[rune(x[6])], qwertyPosition[rune(x[7])]
	// Only the captured five-column geometry overrides ordinary membership.
	if abs(a[0]-b[0]) > 1 || abs(a[1]-b[1]) != 5 {
		return false, 0, false
	}
	ordinary, ok := keyboardSimilarity(x, y)
	matched, confidence = standardTerminalSwap(x, averageWith100(ordinary), ok && ordinary >= 90)
	return matched, confidence, true
}

// lastNameCode is the whole-name match key of a folded last name, cached by
// its text. The standard rules gate on it (standardLastNameGate).
func lastNameCode(folded string) string {
	return lastNameKeyCache.get(folded, func(s string) string { return doubleMetaphone(stripAccents(s), lastNameKeyLength) })
}

// lastNameSuffixes are trailing words the Last Name key skips: "Harlow Jr"
// keys as "Harlow". "Harlow PhD" keys as "phd".
var lastNameSuffixes = map[string]bool{"jr": true, "sr": true, "ii": true, "iii": true, "iv": true}

// lastNameWordKey is the Last Name match key of a folded name: characters
// other than letters, digits and white space are removed ("al-rashid" is
// "alrashid", "harlow, jr." is "harlow jr"), trailing suffix words are
// dropped while another word remains, and the last word is keyed. A last word
// with no letter keys as itself.
func lastNameWordKey(folded string) string {
	return lastNameWordKeyCache.get(folded, func(s string) string {
		words := strings.Fields(strings.Map(func(r rune) rune {
			switch {
			case unicode.IsLetter(r) || unicode.IsDigit(r):
				return r
			case unicode.IsSpace(r):
				return ' '
			}
			return -1
		}, s))
		for len(words) > 1 && lastNameSuffixes[words[len(words)-1]] {
			words = words[:len(words)-1]
		}
		if len(words) == 0 {
			return ""
		}
		last := words[len(words)-1]
		if strings.IndexFunc(last, unicode.IsLetter) < 0 {
			return "#" + last
		}
		return doubleMetaphone(stripAccents(last), lastNameKeyLength)
	})
}

var qwertyPosition = func() map[rune][2]int {
	out := map[rune][2]int{}
	for row, keys := range []string{"qwertyuiop", "asdfghjkl", "zxcvbnm"} {
		for col, r := range keys {
			out[r] = [2]int{row, col}
		}
	}
	return out
}()

// keyboardCost is the Keyboard Distance substitution cost the rows fit: the
// distance between two keys is their column distance, plus two between the
// top and the bottom row; neighbouring keys cost 0.05 and every further step
// 0.08 ("Richardson" with its last letter replaced by each other letter). An
// accent alone costs 0.16, anything else 1.
func keyboardCost(x, y rune) float64 {
	if x == y {
		return 0
	}
	bx, by := []rune(stripAccents(string(x))), []rune(stripAccents(string(y)))
	if len(bx) == 0 || len(by) == 0 {
		return 1
	}
	if bx[0] == by[0] {
		return 0.16
	}
	px, okx := qwertyPosition[bx[0]]
	py, oky := qwertyPosition[by[0]]
	if !okx || !oky {
		return 1
	}
	steps := abs(px[1] - py[1])
	if abs(px[0]-py[0]) == 2 {
		steps += 2
	}
	if steps <= 1 {
		return 0.05
	}
	return 0.08 * float64(steps-1)
}

func abs(x int) int {
	if x < 0 {
		return -x
	}
	return x
}

// keyboardSimilarity compares the strings position by position and divides
// the cost by the longer length; a trailing extra character costs 0.25. ok is
// false for the shapes whose native score the rows do not pin down (three or
// more substituted letters, two with a score under 92, an inserted letter, or
// more than two trailing letters): natively some of those match and some do
// not ("Frank" never finds "Franklin", "Kidde" never finds "Kight"), so Glade
// matches none of them.
func keyboardSimilarity(a, b string) (float64, bool) {
	ra, rb := []rune(a), []rune(b)
	if len(ra) > len(rb) {
		ra, rb = rb, ra
	}
	n := len(rb)
	if n == 0 {
		return 100, true
	}
	cost, different := 0.0, 0
	for i := range ra {
		if ra[i] != rb[i] {
			different++
			step := keyboardCost(ra[i], rb[i])
			if step >= 1 {
				// A space, hyphen or other character that is not a letter key
				// never substitutes: "Smith-Jones" does not find "Smith Jones".
				return 0, false
			}
			cost += step
		}
	}
	extra := len(rb) - len(ra)
	score := 100 * (1 - (cost+0.25*float64(extra))/float64(n))
	switch {
	case extra == 0 && different == 1:
		return score, true
	case extra == 0 && different == 2:
		return score, score >= 92
	case extra > 0 && extra <= 2 && different == 0:
		return score, true
	}
	return score, false
}

// ---- Phone: weighted sections; threshold 80 ----

// phoneParts splits a North American number into country, area, exchange and
// line sections. Letters and digits both count as characters of the number (a
// letter typed for a digit, "312 555 O199", still parses and differs only in
// its line section); separators are dropped. ok is false unless ten characters
// remain, or eleven with the country digit first, and for any other character.
func phoneParts(s string) (country, area, exchange, line string, ok bool) {
	var kept []rune
	for _, r := range strings.ToLower(s) {
		switch {
		case (r >= '0' && r <= '9') || (r >= 'a' && r <= 'z'):
			kept = append(kept, r)
		case strings.ContainsRune("()+-./ \t", r):
		default:
			return "", "", "", "", false
		}
	}
	d := string(kept)
	switch {
	case len(d) == 10:
		return "", d[0:3], d[3:6], d[6:], true
	case len(d) == 11:
		return d[:1], d[1:4], d[4:7], d[7:], true
	}
	return "", "", "", "", false
}

func fuzzyPhone(a, b string, _ bool) (bool, float64) {
	ca, aa, ea, la, okA := phoneParts(a)
	cb, ab, eb, lb, okB := phoneParts(b)
	if !okA || !okB {
		return false, 0
	}
	// The captured vanity shape has country 1, a numeric area, and seven
	// letters. Only admit its fully equivalent digit form; other letter
	// placements and partial matches retain the previous scoring.
	if (ea[0] >= 'a' && phoneKeypadEquivalent(ca, aa, ea, la, cb, ab, eb, lb)) ||
		(eb[0] >= 'a' && phoneKeypadEquivalent(cb, ab, eb, lb, ca, aa, ea, la)) {
		if ca == cb {
			return true, 100
		}
		return true, 90
	}
	if ca != cb && ((ca != "" && ca != "1") || (cb != "" && cb != "1")) {
		// The rows match a number with the NANP country digit 1 to the same
		// number without it (90), but never one with a different country digit,
		// although the weighted score would still reach 90.
		return false, 0
	}
	score := 0.0
	for _, section := range []struct {
		x, y   string
		weight float64
	}{{ca, cb, 10}, {aa, ab, 50}, {ea, eb, 30}, {la, lb, 10}} {
		if section.x == section.y {
			score += section.weight
		}
	}
	return score >= 80, score
}

// phoneKeypadEquivalent covers the captured seven-letter vanity shape.
func phoneKeypadEquivalent(country, area, exchange, line, otherCountry, otherArea, otherExchange, otherLine string) bool {
	if country != "1" || (otherCountry != "" && otherCountry != "1") || area != otherArea {
		return false
	}
	// ITU-T E.161 (02/2001), section 2.2: letter-to-digit keypad layout.
	// https://www.itu.int/rec/T-REC-E.161/en
	const letters = "abcdefghijklmnopqrstuvwxyz"
	const digits = "22233344455566677778889999"
	for i := 0; i < len(area); i++ {
		if area[i] < '0' || area[i] > '9' {
			return false
		}
	}
	for _, section := range [][2]string{{exchange, otherExchange}, {line, otherLine}} {
		for i := 0; i < len(section[0]); i++ {
			j := strings.IndexByte(letters, section[0][i])
			if j < 0 || digits[j] != section[1][i] {
				return false
			}
		}
	}
	return true
}

// ---- ZIP: the segment before the first hyphen, space, period or slash decides ----

// fuzzyZip: NATIVE (phone_city_zip N5.zu1) "12345.6789" and "12345/6789"
// find "12345" and "12345-6789" as a hyphen does.
func fuzzyZip(a, b string, _ bool) (bool, float64) {
	head := func(s string) string {
		s = foldText(s)
		if i := strings.IndexAny(s, "- ./"); i >= 0 {
			return s[:i]
		}
		return s
	}
	// A value under three characters is no postal code and matches nothing,
	// not even itself ("0", "L1"); this guard is a conservative stand-in for
	// the platform's validation, which the probe rows do not cover.
	if len([]rune(strings.TrimSpace(a))) < 3 || len([]rune(strings.TrimSpace(b))) < 3 {
		return false, 0
	}
	if head(a) == head(b) {
		return true, 100
	}
	return false, 0
}

// ---- City: Edit Distance and Exact; threshold 85 ----

const cityKeyLength = 6

// cityKey is the first six letters or digits of the accent-folded value with
// spaces and punctuation removed; pairs with different keys are never scored.
func cityKey(s string) string {
	var out []rune
	for _, r := range stripAccents(foldText(s)) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			out = append(out, r)
		}
		if len(out) == cityKeyLength {
			break
		}
	}
	return string(out)
}

// plainSpaced reports a value whose only white space is ASCII spaces and
// whose characters are all ASCII. NATIVE (phone_city_zip N5.cm1 and N5.szn1):
// between two such values inner spaces count as typed: "Los Angeles" finds
// "Los  Angeles" at 96, not 100, "LosAngeles" no longer finds it, and
// "SW1A 1AA" does not find "SW1A  1AA". Any other pair keeps the folded
// comparison, which no row measures.
func plainSpaced(s string) bool {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 0x80 || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r' {
			return false
		}
	}
	return true
}

// cityTexts is the text the City method compares for a pair (plainSpaced).
func cityTexts(a, b string) (string, string) {
	if plainSpaced(a) && plainSpaced(b) {
		return strings.ToLower(strings.TrimSpace(a)), strings.ToLower(strings.TrimSpace(b))
	}
	return foldText(a), foldText(b)
}

func fuzzyCity(a, b string, _ bool) (bool, float64) {
	x, y := cityTexts(a, b)
	if x == y {
		return true, 100
	}
	if cityKey(x) != cityKey(y) {
		return false, 0
	}
	score := editSimilarity(x, y)
	return score >= 85, averageWith100(score)
}

// ---- Street: weighted sections; threshold 80 ----

// knownStreetSuffixes are the suffix words the native rows treat as the
// street suffix section; every other trailing word stays in the street name.
var knownStreetSuffixes = map[string]string{
	"st": "street", "street": "street", "ave": "avenue", "avenue": "avenue", "rd": "road", "road": "road",
	"blvd": "boulevard", "boulevard": "boulevard", "dr": "drive", "drive": "drive", "ln": "lane", "lane": "lane",
	"cir": "cir", "hwy": "hwy", "crt": "crt",
}

var streetPrefixSuffixes = map[string]bool{"rue": true}

// customStreetSuffixes are the suffix words of the custom Street method: the
// known suffixes and "Aly"/"Alley". NATIVE fuzzy_street_c8/org.tsv qe1: "12
// Hazel Aly" and "12 Hazel Alley" are one street (93), as "Ave" and "Avenue"
// are. The standard rules keep knownStreetSuffixes.
var customStreetSuffixes = func() map[string]string {
	m := map[string]string{"aly": "alley", "alley": "alley"}
	for k, v := range knownStreetSuffixes {
		m[k] = v
	}
	return m
}()

func streetSuffixes(custom bool) map[string]string {
	if custom {
		return customStreetSuffixes
	}
	return knownStreetSuffixes
}

var streetDirections = map[string]bool{"n": true, "s": true, "e": true, "w": true, "ne": true, "nw": true, "se": true, "sw": true, "north": true, "south": true, "east": true, "west": true}

// streetCompassWords are the spelled directionals the custom Street method
// drops between the street number and the name.
var streetCompassWords = map[string]bool{"north": true, "south": true, "east": true, "west": true, "northeast": true, "northwest": true, "southeast": true, "southwest": true}

// streetLongDirections are the compound directional words the base parse keeps
// as the suite after a suffix ("Peachtree St NE" against "Street Northeast").
var streetLongDirections = map[string]bool{"northeast": true, "northwest": true, "southeast": true, "southwest": true}

var streetOrdinals = map[string]string{"first": "1st", "second": "2nd", "third": "3rd", "fourth": "4th", "fifth": "5th", "sixth": "6th", "seventh": "7th", "eighth": "8th", "ninth": "9th", "tenth": "10th"}

// streetNumberWords are the number words the custom Street method reads as
// digits. NATIVE fuzzy_street_c5/org.tsv nw1, fuzzy_street_c4 cs1,
// fuzzy_street_c8/org.tsv qn1 to qn4: "200 Ten St" is "200 10 St", "One Calder
// Plaza" is "1 Calder Plaza" and "Suite Five" is "Suite 5": zero to nineteen
// and the tens from twenty to ninety. "Hundred", "Thousand", "Million",
// "Eleventh" and "Twentieth" stay words.
var streetNumberWords = map[string]string{
	"zero": "0", "one": "1", "two": "2", "three": "3", "four": "4", "five": "5", "six": "6", "seven": "7", "eight": "8",
	"nine": "9", "ten": "10", "eleven": "11", "twelve": "12", "thirteen": "13", "fourteen": "14", "fifteen": "15",
	"sixteen": "16", "seventeen": "17", "eighteen": "18", "nineteen": "19", "twenty": "20", "thirty": "30",
	"forty": "40", "fifty": "50", "sixty": "60", "seventy": "70", "eighty": "80", "ninety": "90",
}

var digitsOnly = regexp.MustCompile(`^[0-9]+$`)

// streetSuiteDesignators are the words the rows drop before a suite value.
var streetSuiteDesignators = map[string]bool{"suite": true, "ste": true, "apt": true, "unit": true, "rm": true, "bldg": true}

// streetFloorDesignators are more suite designators for the custom Street
// method, under NullNotAllowed and Match Blank Fields alike. NATIVE
// duplicate_fuzzy_matching/org.tsv: "600 Montgomery St Fl 3" finds "600
// Montgomery St" at 89 and "600 Montgomery St Fl 4" at 90.
// fuzzy_street_c5/org.tsv nf1: "Rd Fl 3", "Rd Floor 3", "Rd Room 3", "Rd
// Building 3" and "Rd Apartment 3" find the street without the suite;
// "Level", "Lot", "Dept", "No", "PMB", "Stop", "Trailer" and "Space" do not.
// fuzzy_street_c8/org.tsv qg2, qs3: the Match Blank Fields rows read them the
// same way.
var streetFloorDesignators = map[string]bool{"fl": true, "floor": true, "room": true, "building": true, "apartment": true}

// streetCustomSuiteValue reports a suite value of the custom Street method: a
// number, a number with one letter, one letter, or a letter with a number.
// NATIVE fuzzy_street_c8/org.tsv qs1, qs2: "Suite 5", "5A", "A", "A5", "12B"
// and "Five" split; "5th", "1st", "21st", "5 5", "5 B" and "5 Rear" do not.
func streetCustomSuiteValue(w string) bool {
	if w == "" || streetDirections[w] {
		return false
	}
	isDigit := func(c byte) bool { return c >= '0' && c <= '9' }
	isLetter := func(c byte) bool { return c >= 'a' && c <= 'z' }
	allDigits := func(t string) bool {
		if t == "" {
			return false
		}
		for i := 0; i < len(t); i++ {
			if !isDigit(t[i]) {
				return false
			}
		}
		return true
	}
	n := len(w)
	switch {
	case allDigits(w): // 5
		return true
	case n == 1: // A
		return isLetter(w[0])
	case isLetter(w[n-1]) && allDigits(w[:n-1]): // 5A
		return true
	case isLetter(w[0]) && allDigits(w[1:]): // A5
		return true
	case n >= 3 && w[n-2] == '-' && isLetter(w[n-1]) && allDigits(w[:n-2]): // 5-A
		return true
	case n >= 3 && w[1] == '-' && isLetter(w[0]) && allDigits(w[2:]): // A-5
		return true
	}
	return false
}

// streetDigitBeforeLast reports a word with a digit that is not the last word.
func streetDigitBeforeLast(words []string) bool {
	for i := 0; i+1 < len(words); i++ {
		if strings.ContainsAny(words[i], "0123456789") {
			return true
		}
	}
	return false
}

// streetCustomTail reports the words after a suffix that leave it a suffix in
// the custom Street method: nothing, a directional, or a suite.
func streetCustomTail(rest []string) bool {
	if len(rest) > 0 && (streetDirections[rest[0]] || streetLongDirections[rest[0]]) {
		rest = rest[1:]
	}
	switch len(rest) {
	case 0:
		return true
	case 1:
		return streetCustomSuiteValue(rest[0])
	case 2:
		return (streetSuiteDesignators[rest[0]] || streetFloorDesignators[rest[0]]) && streetCustomSuiteValue(rest[1])
	}
	return false
}

// streetSuiteToken reports a trailing token that is a suite value: a number,
// a number with letters ("2B") or a single letter that is not a direction.
func streetSuiteToken(w string) bool {
	if digitsOnly.MatchString(w) {
		return true
	}
	if strings.IndexFunc(w, unicode.IsDigit) >= 0 && len(w) <= 4 {
		return true
	}
	return len(w) == 1 && isAlphaWord(w) && !streetDirections[w]
}

type streetParts struct{ number, name, suffix, suite string }

// streetClean lower-cases the address, removes dots and number signs, turns
// commas into spaces, collapses whitespace and spells ordinal words as numbers.
func streetClean(s string) string { return streetCleanFor(s, false) }

// streetCleanFor is streetClean. For the custom Street method a line break is
// deleted, not turned into a space, and a comma before it goes with it; a
// number sign, slash or colon is deleted where it stands. NATIVE
// fuzzy_street_c4/org.tsv cj1, cj2: "Rd\nSuite 200", "Rd,\nSuite 200" and
// "RdSuite 200" are one street. fuzzy_street_c5/org.tsv nh1, nn1: "Ave#5",
// "Ave/5", "Ave.5" and "Ave:5" are one street and "72/74" is "7274"; a hyphen
// stays ("Ave-5" is another street).
var (
	streetBreakReplacer = strings.NewReplacer(",\r\n", "", ",\n", "", ",\r", "", "\r\n", "", "\n", "", "\r", "")
	streetPunctReplacer = strings.NewReplacer(",", " ", ".", "", "#", "", "/", "", ":", "")
	// streetKeyBreakReplacer reads a line break as a word separator
	// (streetKeyClean); streetSharedPunctReplacer is the punctuation step of
	// the shared cleaning. Both are built once: a duplicate scan cleans two
	// streets for every stored record.
	streetKeyBreakReplacer    = strings.NewReplacer("\r\n", " ", "\n", " ", "\r", " ")
	streetSharedPunctReplacer = strings.NewReplacer(",", " ", ".", "", "#", " ")
)

func streetCleanFor(s string, custom bool) string {
	return streetCleanBreaks(s, custom, true)
}

// streetKeyClean is the custom cleaning with a line break read as a word
// separator, which is how the match key reads the address. NATIVE
// fuzzy_street_c8: "Calder Rd Suite 200" and "Calder Rd\nSuite 200" share a
// key although the break is deleted when the street is scored.
func streetKeyClean(s string) string { return streetCleanBreaks(s, true, false) }

func streetCleanBreaks(s string, custom, deleteBreaks bool) string {
	s = strings.ToLower(s)
	if custom {
		if strings.ContainsAny(s, "\r\n") {
			if deleteBreaks {
				s = streetBreakReplacer.Replace(s)
			} else {
				s = streetKeyBreakReplacer.Replace(s)
			}
		}
		words := strings.Fields(streetPunctReplacer.Replace(streetFoldASCII(s)))
		for i, w := range words {
			if n, ok := streetNumberWords[w]; ok {
				words[i] = n
			}
		}
		return streetCleanWords(strings.Join(words, " "), false)
	}
	return streetCleanWords(s, true)
}

// streetFoldASCII reads the accented letters of a custom Street address as
// their base letters and deletes every other character outside ASCII that is
// not white space. NATIVE fuzzy_street_c8/org.tsv qh1, qh2, qm1, qd1, qd2:
// "Borüden" is "Boruden" (the same for ö, ä, é, è, ñ, ç, å, ê, í and the
// capitals); ß, ø, æ, ł, đ, œ, þ, ð, я, α, 中 and the signs –, ’, ° and € are
// deleted: "Fußweg 5" is "Fuweg 5" and "Hauptstraße Süd" scores as
// "Hauptstrae Sud". The text is lower case.
func streetFoldASCII(s string) string {
	ascii := true
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			ascii = false
			break
		}
	}
	if ascii {
		return s
	}
	var out []rune
	for _, r := range stripAccents(s) {
		if r >= 0x80 && !unicode.IsSpace(r) {
			continue
		}
		out = append(out, r)
	}
	return string(out)
}

func streetCleanWords(s string, hash bool) string {
	if hash {
		s = streetSharedPunctReplacer.Replace(s)
	}
	words := strings.Fields(s)
	for i, w := range words {
		if o, ok := streetOrdinals[w]; ok {
			words[i] = o
		}
	}
	return strings.Join(words, " ")
}

func isAlphaWord(w string) bool {
	for _, r := range w {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return w != ""
}

// parseStreet splits an address into the four scored sections. The number is
// a leading token that starts with a digit; a single letter after it is
// dropped when it is "e" and otherwise kept with the number (the native rows
// match "100 E Main St" to "100 Main St" but not "100 N Main St"). The suffix
// is the first known suffix word after the name; what follows it is a
// directional (dropped) or the suite, whose designator word is dropped. With
// no known suffix a trailing number is the suite and a trailing directional
// is dropped.
func parseStreet(s string) streetParts { return parseStreetFor(s, false) }

var streetPOBox = regexp.MustCompile(`^p ?o ?box ([0-9]+)$`)

// streetGluedNumber is a digit-led first word that is not a street number
// ("41harlan" from "41\nHarlan Ave"); NATIVE cj2 scores it as a name word
// (the equal street reports 88). nn1: "72AB Vane St" reports 100, so two
// letters still make a street number.
var streetGluedNumber = regexp.MustCompile(`^[0-9]+[a-z]{3,}$`)

func parseStreetFor(s string, custom bool) streetParts {
	var p streetParts
	cleaned := streetCleanFor(s, custom)
	if custom {
		if m := streetPOBox.FindStringSubmatch(cleaned); m != nil {
			// NATIVE cp1: "PO Box 410", "P.O. Box 410", "P O Box 410",
			// "PO BOX 410" and "Pobox 410" share one number and one name
			// (95 to 100 between them; 410 and 411 or 4100 never match).
			// "POB 410", "Post Office Box 410" and "Box 410" are plain addresses.
			return streetParts{number: m[1], name: "pobox"}
		}
	}
	words := strings.Fields(cleaned)
	switch {
	case len(words) == 0 || words[0][0] < '0' || words[0][0] > '9':
	case custom && len(words) == 1 && !digitsOnly.MatchString(words[0]):
		// NATIVE fuzzy_street_c9/org.tsv r7: a street of one word that is
		// not a number is all name ("3F" reports 86 for itself, 100 under
		// Match Blank Fields; "12" reports 100).
	case custom && streetGluedNumber.MatchString(words[0]):
	case custom && streetOrdinalWord(words[0]):
		// NATIVE fuzzy_street_c10/org.tsv o4, o5: an ordinal first word is
		// no street number ("3rd Ave" and "Third Ave" report 88 for
		// themselves, as a street with no number does; "101st St" too).
	case custom:
		// NATIVE fuzzy_street_c5/org.tsv nl1, np1, np2, fuzzy_street_c4 cl1
		// to cl3: a directional between the number and a street name with a
		// known suffix is dropped ("71 E Vane St" finds "71 Vane St" at 92 and
		// "71 E E Vane St" at 83; "101 N Park Ln" finds "101 Park Ln N" at
		// 85; sd1, nd4: "500 North Calder Rd" finds "500 Northwest Calder Rd"
		// at 91). Every other single letter stays in the street name. The
		// match key (streetCustomKey) decides which of them keep two streets
		// apart.
		p.number, words = words[0], words[1:]
		// NATIVE fuzzy_street_c10/org.tsv e4: whole numbers after the street
		// number belong to the number when a word follows: "500 Twenty First
		// Street" finds "500 Twenty Tenth Street" at 80 (number "500 20", names
		// "1st" and "10th"), not at 86.
		for len(words) > 1 && streetDigits(words[0]) && !streetAllNumbers(words) {
			p.number, words = p.number+" "+words[0], words[1:]
		}
		if len(words) > 2 && (streetDirections[words[0]] || streetCompassWords[words[0]]) {
			for _, w := range words[2:] {
				if _, ok := streetSuffixes(custom)[w]; ok {
					words = words[1:]
					break
				}
			}
		}
	default:
		p.number, words = words[0], words[1:]
		if len(words) > 1 && len(words[0]) == 1 && isAlphaWord(words[0]) {
			if words[0] != "e" {
				p.number += " " + words[0]
			}
			words = words[1:]
		}
	}
	suffixAt, wholeName := -1, false
	for i := 1; i < len(words); i++ {
		if _, ok := streetSuffixes(custom)[words[i]]; ok {
			if custom && !streetCustomTail(words[i+1:]) {
				// NATIVE fuzzy_street_c8/org.tsv qs6: a known suffix word
				// whose tail is not a suite or directional is a word of the
				// name; the next known suffix word is tried ("Elm Ave Ave
				// A5" has the suffix at the second Ave). A number before
				// the last word of the tail ("Ave 5 5", "Suite 5 B") leaves
				// every word in the name and no suite.
				wholeName = wholeName || streetDigitBeforeLast(words[i+1:])
				continue
			}
			suffixAt = i
			break
		}
	}
	switch {
	case suffixAt < 0 && wholeName:
		// Every known suffix word has a tail with a number before its last
		// word: the street has no suffix and no suite, and every word is
		// the name.
	case !custom && len(words) > 1 && streetPrefixSuffixes[words[0]]:
		// The custom method reads "Rue" as a word of the name (NATIVE
		// fuzzy_street_c8/org.tsv qf1: "7 Rue Royale" against "7 Rue Royal"
		// scores name 90, 92 in all; "12 Rue de Rivoli" finds "12 Rue
		// Rivoli" at 82). The match key still leaves it out.
		p.suffix, words = words[0], words[1:]
	case suffixAt > 0 && custom:
		// NATIVE cu3, fuzzy_street_c8/org.tsv qb3, qs1, qs2: what follows the
		// suffix must be nothing, a directional, or a suite (one value, with
		// or without a designator word). Anything else means the word is
		// not a suffix: the street has no suffix and every word is the
		// name. "77 Wren St Xq" matches no other street; "14 Kingsley Blvd
		// Jr" (name "kingsley blvd jr") finds "14 Kingsley King Jr Blvd" at
		// 72; "200 Elm Ave Suite 5th" and "Suite 5 5" do not split.
		p.suffix = customStreetSuffixes[words[suffixAt]]
		rest := words[suffixAt+1:]
		words = words[:suffixAt]
		if len(rest) > 0 && (streetDirections[rest[0]] || streetLongDirections[rest[0]]) {
			rest = rest[1:]
		}
		switch {
		case len(rest) == 2:
			p.suite = rest[1]
		case len(rest) == 1:
			p.suite = rest[0]
		}
	case suffixAt > 0:
		p.suffix = streetSuffixes(custom)[words[suffixAt]]
		rest := words[suffixAt+1:]
		words = words[:suffixAt]
		if len(rest) > 0 && streetDirections[rest[0]] {
			rest = rest[1:]
		}
		switch {
		case len(rest) >= 2 && streetSuiteDesignators[rest[0]]:
			p.suite = strings.Join(rest[1:], "")
		case len(rest) == 1 || (len(rest) >= 2 && !isAlphaWord(rest[0])):
			p.suite = strings.Join(rest, "")
		default:
			// Any other word ("Fl", "Floor") is not a suite: the rows never
			// match such an address to one without it.
			words = append(words, rest...)
		}
	case custom:
		// NATIVE fuzzy_street_c4/org.tsv cj2, fuzzy_street_c5 nd4: with no
		// known suffix a trailing number is the suite; a number with letters
		// or a single letter is the suite only after a designator word ("Way
		// Suite 2B" but not "AveApt 3B"). A trailing directional stays in the
		// street name ("St N" finds "Str N" at 61, "Str N" finds "St N" at 79).
		if n := len(words); n > 1 && (digitsOnly.MatchString(words[n-1]) || n > 2 && streetCustomSuiteValue(words[n-1]) && (streetSuiteDesignators[words[n-2]] || streetFloorDesignators[words[n-2]])) {
			p.suite, words = words[n-1], words[:n-1]
		}
	default:
		if len(words) > 1 && streetSuiteToken(words[len(words)-1]) {
			p.suite, words = words[len(words)-1], words[:len(words)-1]
		}
		if len(words) > 1 && streetDirections[words[len(words)-1]] {
			words = words[:len(words)-1]
		}
	}
	p.name = strings.Join(words, " ")
	return p
}

// streetAllNumbers reports words that are whole numbers only.
func streetAllNumbers(words []string) bool {
	for _, w := range words {
		if !streetDigits(w) {
			return false
		}
	}
	return true
}

const streetKeyLength = 5

func streetKey(p streetParts) string {
	name := []rune(p.name)
	if len(name) > streetKeyLength {
		name = name[:streetKeyLength]
	}
	return p.number + "|" + string(name)
}

// streetSections is the weighted section score: number 20 (exact), name 50
// (edit similarity), suffix 15 (exact), suite 15 (exact), averaged over the
// sections present on both sides. A section present on one side only is left
// out; a street number blank on both sides scores 0 under NullNotAllowed and
// 20 under MatchBlanks.
func streetSections(pa, pb streetParts, matchBlanks bool) float64 {
	return streetSectionsWith(pa, pb, matchBlanks, editSimilarity)
}

// streetCandidateSimilarity is the edit similarity the custom Street method
// reports: 0 when the edit distance equals the candidate's length, whatever
// the stored value's length. NATIVE fuzzy_street_c4/org.tsv cu1, cu3 and
// duplicate_fuzzy_matching: "15 Elm" finds "15 Elm Drive" at 50 and "77 Wren
// St" finds "77 Wren Str" at 60, while the reverse pairs report 75 and 78.
func streetCandidateSimilarity(candidate, stored string) float64 {
	ra, rb := []rune(candidate), []rune(stored)
	longest := max(len(ra), len(rb))
	if longest == 0 {
		return 100
	}
	distance := levenshtein(ra, rb)
	if distance == len(ra) {
		return 0
	}
	return 100 * (1 - float64(distance)/float64(longest))
}

func streetSectionsWith(pa, pb streetParts, matchBlanks bool, similarity func(a, b string) float64) float64 {
	return streetSectionScore(pa, pb, matchBlanks, similarity, false)
}

// streetSectionScore is streetSections. With numberAlways (the custom method)
// a street number or a street name on one side only counts as a mismatch
// instead of being left out. NATIVE fuzzy_street_c4/org.tsv cj2: "41 Harlan
// Ave" finds "41\nHarlan Ave" (no street number) at 77, not 87.
// fuzzy_street_c9/org.tsv r7: "12" finds neither "12 St" nor "12 Park" nor "12
// Park St", while "12 Park" finds "12 Park St" at 85 (a suffix on one side is
// left out).
func streetSectionScore(pa, pb streetParts, matchBlanks bool, similarity func(a, b string) float64, numberAlways bool) float64 {
	num, den := 0.0, 0.0
	for _, section := range []struct {
		x, y   string
		weight float64
		name   bool
		number bool
	}{{pa.number, pb.number, 20, false, true}, {pa.name, pb.name, 50, true, false}, {pa.suffix, pb.suffix, 15, false, false}, {pa.suite, pb.suite, 15, false, false}} {
		switch {
		case section.x == "" && section.y == "":
			if section.number {
				den += section.weight
				if matchBlanks {
					num += section.weight
				}
			}
		case section.x == "" || section.y == "":
			if (section.number || section.name) && numberAlways {
				den += section.weight
			}
		case section.name:
			den += section.weight
			num += section.weight * similarity(section.x, section.y) / 100
		default:
			den += section.weight
			if section.x == section.y {
				num += section.weight
			}
		}
	}
	if den == 0 {
		return 100
	}
	return 100 * num / den
}

// fuzzyStreet: the match key is the street number plus the first five
// characters of the name. The weighted section score decides at 80; the rule
// confidence averages that score with the edit similarity of the cleaned
// strings.
func fuzzyStreet(a, b string, matchBlanks bool) (bool, float64) {
	x, y := streetClean(a), streetClean(b)
	if x == y {
		return true, 100
	}
	if confidence, ok := streetAbbreviationPair(x, y); ok {
		return true, confidence
	}
	pa, pb := parseStreet(a), parseStreet(b)
	sections := streetSections(pa, pb, matchBlanks)
	similarity := editSimilarity(x, y)
	confidence := (sections + similarity) / 2
	if streetKey(pa) != streetKey(pb) {
		return false, confidence
	}
	if streetTwoEdits(pa, pb) {
		return false, confidence
	}
	if pa.number == "" && pb.number == "" && pa.name != pb.name && editSimilarity(pa.name, pb.name) < 80 {
		// With no street number to anchor the pair the name must be as alike
		// as the overall threshold demands: the rows never match
		// "Calle 8 No 5" to "Calle 8 Numero 5" (name similarity 71) although the
		// weighted score reaches 80 under MatchBlanks.
		return false, confidence
	}
	return sections >= 80, confidence
}

// streetAbbreviationPairs are two spellings of a street's last word that the
// rows match to each other whatever the section model says, with the native
// confidence (the rows give 77 and 59 by direction for Oak, 80 for Pine; Glade
// reports one value for both directions).
var streetAbbreviationPairs = map[[2]string]float64{
	orderedPair("ct", "crt"): 70,
}

// streetAbbreviationPair reports two cleaned addresses that are the same but
// for their last word, when that pair is in streetAbbreviationPairs.
func streetAbbreviationPair(x, y string) (float64, bool) {
	wx, wy := strings.Fields(x), strings.Fields(y)
	if len(wx) < 3 || len(wx) != len(wy) || strings.Join(wx[:len(wx)-1], " ") != strings.Join(wy[:len(wy)-1], " ") {
		return 0, false
	}
	confidence, ok := streetAbbreviationPairs[orderedPair(wx[len(wx)-1], wy[len(wy)-1])]
	return confidence, ok
}

// streetTwoEdits reports names that differ in both the street name proper and
// the unknown trailing word ("Harbor Lp" against "Harbour Loop"). The rows
// match a differing trailing word when the rest is identical ("Maple Cv"
// against "Maple Cove") and a differing street name when the suffix is a
// known suffix word, but never both at once.
func streetTwoEdits(pa, pb streetParts) bool {
	wx, wy := strings.Fields(pa.name), strings.Fields(pb.name)
	if len(wx) < 2 || len(wx) != len(wy) || wx[len(wx)-1] == wy[len(wy)-1] {
		return false
	}
	return strings.Join(wx[:len(wx)-1], " ") != strings.Join(wy[:len(wy)-1], " ")
}

// ---- Company Name: Acronym, Exact, Syllable Alignment; threshold 70 ----

// companyDesignators are the legal designators the native rows remove when
// they are the last word (one only).
var companyDesignators = map[string]bool{
	"inc": true, "incorporated": true, "corp": true, "corporation": true, "co": true, "company": true, "ltd": true, "limited": true,
	"llc": true, "llp": true, "lp": true, "plc": true, "gmbh": true, "ag": true, "sa": true, "enterprises": true, "na": true,
}

var companyStopWords = map[string]bool{"and": true, "of": true, "the": true}

// companyWords lower-cases the name, folds a free-standing "&" to "and" and
// hyphens to spaces, drops punctuation, a leading "the", one trailing
// designator and the stop words. An "&" or "/" inside a word stays part of it:
// natively "AT&T" does not match "AT and T" and "Barnes&Noble" does not match
// "Barnes & Noble".
func companyWords(s string) []string {
	tokens := strings.Fields(strings.ReplaceAll(strings.ToLower(s), "-", " "))
	words := make([]string, 0, len(tokens))
	for _, token := range tokens {
		if token == "&" {
			words = append(words, "and")
			continue
		}
		if companyCleanASCII(token) {
			words = append(words, token)
			continue
		}
		var clean []rune
		for _, r := range token {
			if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '&' || r == '/' {
				clean = append(clean, r)
			}
		}
		if len(clean) > 0 {
			words = append(words, string(clean))
		}
	}
	if len(words) > 1 && words[0] == "the" {
		words = words[1:]
	}
	if len(words) > 1 && companyDesignators[words[len(words)-1]] {
		words = words[:len(words)-1]
	}
	stop := 0
	for _, w := range words {
		if companyStopWords[w] {
			stop++
		}
	}
	if stop == 0 || stop == len(words) {
		return words
	}
	out := make([]string, 0, len(words)-stop)
	for _, w := range words {
		if !companyStopWords[w] {
			out = append(out, w)
		}
	}
	return out
}

// companyCleanASCII reports a token of lower-case ASCII letters, digits, "&"
// and "/" only: companyWords keeps it as it is.
func companyCleanASCII(token string) bool {
	for i := 0; i < len(token); i++ {
		c := token[i]
		if !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9' || c == '&' || c == '/') {
			return false
		}
	}
	return true
}

// companyKey is the match key: the first five characters of each of the first
// two words (help notation "Company (2,5)").
func companyKey(words []string) string {
	prefix := func(w string) (string, bool) {
		w = w[:min(len(w), 5)]
		return w, isASCII(w)
	}
	switch len(words) {
	case 0:
		return ""
	case 1:
		if p, ok := prefix(words[0]); ok {
			return p
		}
	default:
		p, ok := prefix(words[0])
		q, ok2 := prefix(words[1])
		if ok && ok2 {
			return p + q
		}
	}
	var b strings.Builder
	for i, w := range words {
		if i == 2 {
			break
		}
		r := []rune(w)
		if len(r) > 5 {
			r = r[:5]
		}
		b.WriteString(string(r))
	}
	return b.String()
}

var companyWordPairSet = func() map[[2]string]float64 {
	out := map[[2]string]float64{}
	for _, pair := range companyWordPairs {
		out[[2]string{pair.a, pair.b}] = pair.forward
		out[[2]string{pair.b, pair.a}] = pair.reverse
	}
	return out
}()

// companyCollapse folds doubled letters inside a word ("goldmann" is
// "goldman"); the rows score such spellings as identical.
func companyCollapse(w string) string {
	var out []rune
	r := []rune(w)
	for i, c := range r {
		if len(out) > 0 && out[len(out)-1] == c && i < len(r)-1 {
			continue
		}
		out = append(out, c)
	}
	return string(out)
}

// companyWordMatch compares two words: identical, equal after folding doubled
// letters, or a pair the native rows prove (companyWordPairs; x is the
// candidate's word). The second result is the word's match confidence.
func companyWordMatch(x, y string) (bool, float64) {
	if x == y || companyCollapse(x) == companyCollapse(y) {
		return true, 100
	}
	if confidence, ok := companyWordPairSet[[2]string{x, y}]; ok {
		return true, confidence
	}
	return false, 0
}

// companyAlignedWords are the words whose spellings the Syllable Alignment
// rows keep apart whatever their common prefix: natively "Northern Partner"
// finds neither "Northern Partnerly" nor itself in a one-word name, and "Partners"
// never finds "Partner".
var companyAlignedWords = map[string]bool{"partner": true, "partners": true}

// companySecondWordMatch is the native Syllable Alignment result for the
// second word of two names with the same first word (NATIVE: 285 pairs of the
// two-word probe rows, all API 67.0): two words that share their first six
// letters match at 100 when one is the other with more letters after the
// prefix, or when they differ in a single letter after it ("Baseline" and
// "Baselinu", "Silverline" and "Silverlane", "Hospital" and "Hospitality").
// Words of five letters or fewer never match, nor do words that differ in two
// letters ("Hillcrest" and "Hillcrset").
func companySecondWordMatch(x, y string) bool {
	rx, ry := []rune(x), []rune(y)
	if companyAlignedWords[x] || companyAlignedWords[y] {
		return false
	}
	common := 0
	for common < len(rx) && common < len(ry) && rx[common] == ry[common] {
		common++
	}
	if common < 6 {
		return false
	}
	if common == len(rx) || common == len(ry) {
		return true
	}
	return len(rx) == len(ry) && common == len(rx)-1
}

// companyOneWayPairs are one-word company names the native rows match in one
// direction only at a confidence the plural rule does not give: the name is
// the candidate's, the second word the existing record's (a company alias, not
// a plural: "Walgreen" finds "Walgreens" at 100, "Walgreens" does not find
// "Walgreen").
var companyOneWayPairs = map[[2]string]float64{{"walgreen", "walgreens"}: 100}

// companySingleWordMatch is the one-direction plural of a one-word name:
// natively "Nordstrom" finds "Nordstroms" and "Nordstroms" does not find
// "Nordstrom" (NATIVE: every such pair of six letters or more in the one-word
// probe rows except "Prairie" and "Partner"; words of five letters or fewer
// never match). The confidence grows with the length of the word and is higher
// for a word that ends in a vowel sound.
func companySingleWordMatch(candidate, existing string) (bool, float64) {
	if confidence, ok := companyOneWayPairs[[2]string{candidate, existing}]; ok {
		return true, confidence
	}
	if existing != candidate+"s" || len([]rune(candidate)) < 6 || companyAlignedWords[candidate] || candidate == "prairie" {
		return false, 0
	}
	switch candidate[len(candidate)-1] {
	case 'a', 'y', 'w', 's':
		return true, 98
	}
	switch n := len([]rune(candidate)); {
	case n <= 6:
		return true, 91
	case n <= 8:
		return true, 93
	case n <= 10:
		return true, 95
	}
	return true, 96
}

// companyAlignedName reports a name that folds to one of companyAlignedWords.
func companyAlignedName(s string) bool {
	t := strings.TrimSpace(s)
	if !isASCII(t) {
		return companyAlignedWords[foldText(s)]
	}
	return strings.EqualFold(t, "partner") || strings.EqualFold(t, "partners")
}

// foldedEqual is foldText(a) == foldText(b) without building either string
// for ASCII text.
func foldedEqual(a, b string) bool {
	if !isASCII(a) || !isASCII(b) {
		return foldText(a) == foldText(b)
	}
	space := func(c byte) bool { return c == ' ' || c == '\t' || c == '\n' || c == '\v' || c == '\f' || c == '\r' }
	lower := func(c byte) byte {
		if c >= 'A' && c <= 'Z' {
			return c + 'a' - 'A'
		}
		return c
	}
	i, j := 0, 0
	for {
		for i < len(a) && space(a[i]) {
			i++
		}
		for j < len(b) && space(b[j]) {
			j++
		}
		if i == len(a) || j == len(b) {
			return i == len(a) && j == len(b)
		}
		for i < len(a) && j < len(b) && !space(a[i]) && !space(b[j]) {
			if lower(a[i]) != lower(b[j]) {
				return false
			}
			i++
			j++
		}
		if (i < len(a) && !space(a[i])) || (j < len(b) && !space(b[j])) {
			return false
		}
	}
}

func isASCII(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func legacyFuzzyCompany(a, b string, _ bool) (bool, float64) {
	if companyAlignedName(a) || companyAlignedName(b) {
		// NATIVE: a name that is only "Partner" matches nothing, itself included.
		return false, 0
	}
	if foldedEqual(a, b) {
		return true, 100
	}
	// The cached words are shared: read them only.
	ia, ib := companyKeyOf(a), companyKeyOf(b)
	wa, wb := ia.words, ib.words
	if len(wa) == 0 || len(wb) == 0 {
		return false, 0
	}
	ja, jb := ia.joined, ib.joined
	if strings.Contains(a, "-") != strings.Contains(b, "-") {
		// A hyphenated name matches only the same words spelled with a space;
		// the rows score it 97 or 100 by direction and match neither its
		// acronym nor the same words plus one.
		return ja == jb, 97
	}
	if companyAcronyms[ja] == jb || companyAcronyms[jb] == ja {
		return true, 100
	}
	if len(wa) == 1 && len(wb) == 1 && ia.key == ib.key {
		if ok, c := companySingleWordMatch(wa[0], wb[0]); ok {
			return true, c
		}
	}
	if ia.key != ib.key {
		return false, 0
	}
	confidence := 100.0
	same := true
	for i := 0; i < min(len(wa), len(wb)); i++ {
		ok, c := companyWordMatch(wa[i], wb[i])
		if !ok && i == 1 && wa[0] == wb[0] && companySecondWordMatch(wa[i], wb[i]) {
			ok, c = true, 100
		}
		if !ok {
			if i < 2 {
				return false, 0
			}
			same = false
			continue
		}
		confidence = min(confidence, c)
	}
	if len(wa) == len(wb) && same {
		return true, confidence
	}
	if len(wa) >= 2 && len(wb) >= 2 {
		// The rows score one extra or different third word at 91, one extra
		// fourth word after three equal words at 93, and reject two or more
		// extra words.
		switch {
		case max(len(wa), len(wb)) == 3:
			return true, 91
		case same && min(len(wa), len(wb)) == 3 && max(len(wa), len(wb)) == 4:
			return true, 93
		}
	}
	return false, 0
}

// fuzzyCompany combines the captured directional scorer with its match key.
// A pair outside the fitted routes, or one the fitted scorer rejects, keeps
// the word scorer's decision.
func fuzzyCompany(a, b string, matchBlanks bool) (bool, float64) {
	if a == b && companySimpleExact(a) {
		return true, 100
	}
	ma, mb := companyMarks(a), companyMarks(b)
	if (ma|mb)&companyMarkQualifier != 0 {
		if x, y, ok := companySpacedAsDash(a, b); ok {
			return fuzzyCompany(x, y, matchBlanks)
		}
		if !companySlashKeys(a, b) {
			return false, 0
		}
	}
	ok, score := fuzzyCompanyRouted(a, b, matchBlanks, ma, mb)
	if ok {
		if score < 100 && (ma&mb)&companyMarkQualifier != 0 && companyVowelParenScore(a, b) {
			return true, 100
		}
		return ok, score
	}
	if companyDoubledEqual(a, b) {
		return true, 100
	}
	if (ma|mb)&(companyMarkDigit|companyMarkJoin|companyMarkQualifier) == 0 {
		// NATIVE company_name_fit/org.tsv R4_stem: a first word followed only
		// by a legal designator is a one-word name and does not match this
		// way ("Vexlets Inc" does not find "Vexlet Inc" or "Vexlet LLC").
		if stemmed, ok := companyStemFirstWord(a, b); ok && len(companyWords(a)) >= 2 && len(companyWords(b)) >= 2 {
			if matched, confidence := legacyFuzzyCompany(stemmed, b, matchBlanks); matched {
				return true, confidence
			}
		}
	}
	if (ma|mb)&companyMarkQualifier != 0 {
		if confidence, matched := companySlashScore(a, b); matched {
			return true, confidence
		}
	}
	if (ma^mb)&companyMarkJoin != 0 && (ma|mb)&companyMarkDigit == 0 {
		// One side is the dash form, the other plain and without a hyphen.
		dashed := a
		if mb&companyMarkJoin != 0 {
			dashed = b
		}
		if companySpacedHyphen(dashed) {
			if confidence, matched := companyVowelSuffixScore(a, b); matched {
				return true, confidence
			}
		}
	}
	return ok, score
}

// companyDoubledEqual reports two multiword ASCII names that differ only by
// doubled letters in the first word. NATIVE company_name_fit/org.tsv D1_jn:
// "Vexlett Packard" and "Vexlet Packard" match at 100 in both directions;
// NATIVE fuzzy_matching rows reject "Pioneer Atlas" and "Pioneer Atlass".
func companyDoubledEqual(a, b string) bool {
	if a == b {
		return false
	}
	i, j, skipped := 0, 0, false
	var pa, pb byte
	firstA, firstB := true, true
	next := func(s string, k *int, prev *byte, first *bool) (byte, bool) {
		for *k < len(s) {
			c := s[*k]
			*k++
			if c >= 0x80 {
				return 0, false
			}
			if c >= 'A' && c <= 'Z' {
				c += 'a' - 'A'
			}
			if c == ' ' || c == '-' {
				*first = false
			}
			if c == *prev && c >= 'a' && c <= 'z' && *first {
				skipped = true
				continue
			}
			*prev = c
			return c, true
		}
		return 0, true
	}
	for {
		x, okA := next(a, &i, &pa, &firstA)
		y, okB := next(b, &j, &pb, &firstB)
		if !okA || !okB || x != y {
			return false
		}
		if x == 0 && i >= len(a) && j >= len(b) {
			return skipped && strings.ContainsAny(a, " -") && strings.ContainsAny(b, " -") && companyKeyPrefix(a, b)
		}
	}
}

func fuzzyCompanyRouted(a, b string, matchBlanks bool, ma, mb int) (bool, float64) {
	route := companyFitRouteMarked(a, b, ma, mb)
	if route == companyRouteNone {
		return legacyFuzzyCompany(a, b, matchBlanks)
	}
	same, wordsA, wordsB := companyFitKeys(a, b)
	if route == companyRouteJoin && !companyJoinWords(a, wordsA, wordsB) {
		route = companyRouteNone
		if companyQualifierPair(a, b) {
			route = companyRouteQualifier
		}
	}
	if route != companyRouteNone && same {
		if raw := companyFitScore(a, b); raw >= companyFitThreshold && companyFitCovered(route, a, b) {
			if route == companyRouteJoin {
				if matched, _ := legacyFuzzyCompany(a, b, matchBlanks); !matched {
					return true, companyJoinConfidence(a, b, raw)
				}
			}
			return true, math.Max(91, raw)
		}
	}
	return legacyFuzzyCompany(a, b, matchBlanks)
}

// A multiword ASCII name is an exact match in both scorers. Single-word
// names keep the existing Partner exception.
func companySimpleExact(s string) bool {
	if len(s) < 3 || s[0] == ' ' || s[len(s)-1] == ' ' {
		return false
	}
	space := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c == ' ' {
			space = true
		} else if !(c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9') {
			return false
		}
	}
	return space
}

// ---- Title: Acronym, Exact, Kullback-Leibler distance; threshold 50 ----

// NATIVE (fuzzcap-C9 c9t4): "&" is a stop word of its own: "Zorp & Quill"
// equals "Zorp and Quill" by content and scores 93, not 100, against "Quill and
// Zorp".
var titleStopWords = map[string]bool{"and": true, "of": true, "the": true, "&": true}

func titleWords(s string) []string {
	fields := strings.Fields(strings.ToLower(s))
	// The words usually are the fields; an ampersand split needs more room.
	out := fields[:0]
	if strings.IndexByte(s, '&') >= 0 {
		out = make([]string, 0, len(fields)+2)
	}
	for _, w := range fields {
		// NATIVE (fuzzcap-B3 tt1): dots inside a word do not count ("C.E.O."
		// is "ceo", "V.P." is "vp").
		if w = strings.ReplaceAll(strings.Trim(w, "."), ".", ""); titleTrim(w) == "" {
			continue
		}
		// NATIVE (fuzzy_title_variant f1t3): an ampersand at the edge of a
		// word is a word of its own ("Chief& Officer" and "Chief &Officer"
		// are "Chief & Officer"); inside a word it stays ("Chief&Officer").
		if len(w) > 1 && w[0] == '&' {
			out = append(out, "&")
			w = w[1:]
		}
		if len(w) > 1 && w[len(w)-1] == '&' {
			out = append(out, w[:len(w)-1], "&")
			continue
		}
		out = append(out, w)
	}
	// A comma inside the title stays on its word and blocks prefix matches
	// (NATIVE fuzzcap-B3 tt2 and fuzzcap-C9 c9t4: "Director, IT" does not
	// find "Director/IT", "Zorp, Quill" does not find "Zorps Quill"); a
	// trailing comma of the whole title is dropped ("Zorp Quill," finds
	// "Zorp Quills"). titleTrim removes the comma for every other comparison.
	if n := len(out); n > 0 {
		out[n-1] = titleTrim(out[n-1])
	}
	return out
}

// titleNoPrefix are words that never equal a longer word they start: the rows
// give "IT Manager" and "Italy Manager" one shared word, not two, although
// "Pr Manager" matches "Product Manager" and "Principal Staff Software
// Engineer".
var titleNoPrefix = map[string]bool{"it": true}

// titleWordEqual treats a word as equal to any word it starts (two letters
// or more): "Dir" and "Director", "Eng" and "Engineering".
func titleWordEqual(a, b string) bool { return titleWordEqualRaw(a, b) }

// titleWordEqualRaw compares the words as given: equal, one starts the
// other, or one edit apart when the longer has six characters or more.
// NATIVE (duplicate_fuzzy_matching Holdout3 and Holdout4 Title rows,
// fuzzy_title_variant f2t1): "Coordinator" equals "Coordinater", "Assistant"
// "Asistant", "Zorps" "Zorpas" and "Alphas" "Alphs"; "Alpha" does not equal
// "Alphs" and "CEO" does not equal "CFO". A comma or semicolon inside the
// title stays on its word and counts as a character (c9t4b, tt2, f1t3, f2t1):
// "Alpha," equals "Alphas" and "Alpha;", "Alph," equals "Alpha,"; "Zorp,"
// equals neither "Zorps" nor "Zorp;", "Alph," does not equal "Alpha", and
// "Director," does not equal "Director/IT".
func titleWordEqualRaw(a, b string) bool {
	if a == b || titlePrefixEqual(a, b) {
		return true
	}
	return max(len(a), len(b)) >= 6 && titleOneEdit(a, b)
}

// titleOneEdit: one substitution, insertion or deletion turns a into b.
func titleOneEdit(a, b string) bool {
	if len(a) > len(b) {
		a, b = b, a
	}
	if len(b)-len(a) > 1 {
		return false
	}
	i := 0
	for i < len(a) && a[i] == b[i] {
		i++
	}
	if len(a) == len(b) {
		return i < len(a) && a[i+1:] == b[i+1:]
	}
	return a[i:] == b[i+1:]
}

// titlePrefixEqual: one word starts the other, two letters or more.
func titlePrefixEqual(a, b string) bool {
	if min(len(a), len(b)) < 2 || titleNoPrefix[a] || titleNoPrefix[b] {
		return false
	}
	return strings.HasPrefix(a, b) || strings.HasPrefix(b, a)
}

func titleHasEqual(w string, words []string) bool {
	for _, v := range words {
		if titleWordEqual(w, v) {
			return true
		}
	}
	return false
}

func titleHasWords(words []string, want ...string) bool {
	for _, w := range want {
		found := false
		for _, v := range words {
			if titleTrim(v) == w {
				found = true
			}
		}
		if !found {
			return false
		}
	}
	return true
}

func titleAllStop(words []string) bool {
	for _, w := range words {
		if t := titleTrim(w); !titleStopWords[t] && t != "it" {
			return false
		}
	}
	return true
}

// titleStopOverlap scores two titles whose content words share nothing by
// the words they do share, which are stop words. NATIVE (fuzzy_title_variant
// c9t4a, tt2, f1t1, f2t2), by candidate word count, existing word count and
// shared words. A candidate inside the existing title: "of" finds "of the"
// at 91, "Zorp of Quill" at 89 and a longer title at 88; "of it" finds "Zorp
// of IT" at 94. The existing title inside the candidate: "Zorp of IT" finds
// "of it" at 91; a one-word title is never found this way ("Zorp IT" does
// not find "IT"). Neither inside the other: "Director of IT" finds "Head of
// IT" at 88, and "Director IT" does not. Every other count is no match.
func titleStopOverlap(wa, wb []string) (bool, float64) {
	la, lb := len(wa), len(wb)
	if la > 5 || lb > 5 {
		if la != 1 {
			return false, 0
		}
		lb = 5
	}
	shared := 0
	for _, w := range wa {
		if titleHasWords(wb, titleTrim(w)) {
			shared++
		}
	}
	if shared == 0 || shared > 4 {
		return false, 0
	}
	confidence := titleStopOverlapConfidence[la][lb][shared]
	return confidence != 0, float64(confidence)
}

// titleStopOverlapConfidence[candidate words][existing words][shared words].
var titleStopOverlapConfidence = [6][6][5]uint8{
	1: {2: {1: 91}, 3: {1: 89}, 4: {1: 88}, 5: {1: 88}},
	2: {3: {2: 94}, 4: {2: 91}, 5: {2: 90}},
	3: {2: {2: 91}, 3: {2: 88}, 4: {3: 95}, 5: {3: 93}},
	4: {3: {3: 93}, 4: {3: 90}, 5: {3: 88, 4: 96}},
	5: {3: {3: 89}, 4: {4: 95}},
}

func titleTrim(w string) string { return strings.Trim(w, ",") }

// titleContent drops the stop words. NATIVE (fuzzcap-B3 tt2): "it" is dropped
// too ("Director IT", "Director, IT" and "IT Director" are "Director" at 100)
// unless nothing else is left ("IT" against "Information Technology").
func titleContent(words []string) []string {
	dropped := false
	for _, w := range words {
		if t := titleTrim(w); titleStopWords[t] || t == "it" {
			dropped = true
			break
		}
	}
	if !dropped {
		return words
	}
	out := make([]string, 0, len(words))
	for _, w := range words {
		if !titleStopWords[titleTrim(w)] && titleTrim(w) != "it" {
			out = append(out, w)
		}
	}
	if len(out) == 0 {
		for _, w := range words {
			if !titleStopWords[titleTrim(w)] {
				out = append(out, w)
			}
		}
	}
	return out
}

// titleSameWords: the two word lists are equal, a comma aside.
func titleSameWords(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] && titleTrim(a[i]) != titleTrim(b[i]) {
			return false
		}
	}
	return true
}

// titleSameWordSet: the two word lists hold the same words, a comma aside,
// in any order.
func titleSameWordSet(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	if titleSameWords(a, b) {
		return true
	}
	for _, w := range a {
		w = titleTrim(w)
		inA, inB := 0, 0
		for i := range a {
			if titleTrim(a[i]) == w {
				inA++
			}
			if titleTrim(b[i]) == w {
				inB++
			}
		}
		if inA != inB {
			return false
		}
	}
	return true
}

// titleIsInitials: text is the first letters of words, a comma aside.
func titleIsInitials(text string, words []string) bool {
	text = titleTrim(text)
	if len(text) != len(words) {
		return false
	}
	for i, w := range words {
		if w[0] != text[i] {
			return false
		}
	}
	return true
}

// titleJoin concatenates words without their commas.
func titleJoin(words []string) string {
	if len(words) == 1 {
		return titleTrim(words[0])
	}
	var b strings.Builder
	for _, w := range words {
		b.WriteString(titleTrim(w))
	}
	return b.String()
}

// titleReordered: two shared-word lists of one length differ in order.
func titleReordered(a, b []string) bool {
	for i := range a {
		if !titleWordEqual(a[i], b[i]) {
			return true
		}
	}
	return false
}

// titleConfidence is the rule confidence the native rows report by shared
// word count and the larger content-word count, for shared words in the same
// order. The Title item score is twice the confidence less 100 (the rows pair
// Title with an exact Last Name item).
var titleConfidence = map[[2]int]float64{{1, 2}: 93, {1, 3}: 89, {1, 4}: 88, {2, 3}: 95, {2, 4}: 93, {2, 5}: 91, {3, 4}: 97, {3, 5}: 94, {4, 5}: 97}

// titleConfidenceSix extends titleConfidence to six content words, by shared
// word count (NATIVE duplicate_fuzzy_panels, every direction).
var titleConfidenceSix = map[int]float64{2: 89, 3: 93, 4: 95, 5: 98}

// titleShortSide is the confidence of the one-direction matches of a short
// candidate title inside a five or six word existing title, by shared and
// larger word count.
var titleShortSide = map[[2]int]float64{{1, 5}: 88, {2, 5}: 90, {2, 6}: 89}

// uniqueWords drops repeated words. A title has few words, so it compares
// them directly and returns words itself when none repeats.
func uniqueWords(words []string) []string {
	for i := 1; i < len(words); i++ {
		if !slices.Contains(words[:i], words[i]) {
			continue
		}
		out := append(make([]string, 0, len(words)-1), words[:i]...)
		for _, w := range words[i+1:] {
			if !slices.Contains(out, w) {
				out = append(out, w)
			}
		}
		return out
	}
	return words
}

// fuzzyTitle returns the rule confidence directly (see titleConfidence); the
// caller converts it to the item score.
func fuzzyTitle(a, b string, _ bool) (bool, float64) {
	wa, wb := titleWords(a), titleWords(b)
	if len(wa) == 0 || len(wb) == 0 {
		return foldText(a) == foldText(b), 100
	}
	ca, cb := titleContent(wa), titleContent(wb)
	stopA, stopB := titleAllStop(wa), titleAllStop(wb)
	if !stopA && !stopB && titleSameWords(ca, cb) || titleSameWordSet(wa, wb) {
		return true, 100
	}
	if stopA || stopB {
		// A title of stop words alone has no content. Its acronym still
		// counts ("IT" finds "Information Technology"); otherwise the shared
		// stop words decide.
		if stopA != stopB && (stopA && len(cb) > 1 && titleIsInitials(titleJoin(ca), cb) || stopB && len(ca) > 1 && titleIsInitials(titleJoin(cb), ca)) {
			return true, 100
		}
		return titleStopOverlap(wa, wb)
	}
	if len(ca) == 1 && len(cb) == 1 {
		// Two one-word titles need an exact match. NATIVE (fuzzcap-B3 tt2,
		// fuzzy_title_variant f1t2): when a title has more words (only stop
		// words, such as "of" and "IT", count as nothing), the words may
		// differ by a prefix: "Dir" finds "Director of IT", "Director/IT"
		// finds "Director IT" and "Zorp and" finds "Zorps".
		if (len(wa) > 1 || len(wb) > 1) && titleWordEqualRaw(ca[0], cb[0]) {
			return true, 100
		}
		return titleStopOverlap(wa, wb)
	}
	if len(cb) > 1 && titleIsInitials(titleJoin(ca), cb) || len(ca) > 1 && titleIsInitials(titleJoin(cb), ca) {
		return true, 100
	}
	var sharedA, sharedB []string
	for _, w := range ca {
		if titleHasEqual(w, cb) {
			sharedA = append(sharedA, w)
		}
	}
	for _, w := range cb {
		if titleHasEqual(w, ca) {
			sharedB = append(sharedB, w)
		}
	}
	ua, ub := uniqueWords(ca), uniqueWords(cb)
	// The shared count is the smaller of the two directions so that the score
	// does not depend on which title is the candidate ("Attorney" and
	// "Attorney at Law": "at" starts "attorney" only when read from one side).
	k := min(len(uniqueWords(sharedA)), len(uniqueWords(sharedB)))
	if k == 0 {
		return titleStopOverlap(wa, wb)
	}
	if k == len(ua) && k == len(ub) && len(sharedB) == len(cb) {
		// The same content words in a different order: exact unless a stop
		// word made the sorted word lists differ.
		if len(wa) == len(ca) && len(wb) == len(cb) {
			return true, 100
		}
		// NATIVE (fuzzy_title_variant f1t2): the same order is exact
		// whatever the stop words ("Zorp Quill" finds "Zorps and Quill").
		if len(sharedA) == len(sharedB) && !titleReordered(sharedA, sharedB) {
			return true, 100
		}
		// NATIVE (fuzzcap-C9 c9t4): 94 when the candidate has no stop word
		// ("Zorp Quill" finds "Quill of Zorp"), 93 when it has one.
		if len(wa) == len(ca) {
			return true, 94
		}
		return true, 93
	}
	sameOrder := len(sharedA) > 0
	for i := 0; i < min(len(sharedA), len(sharedB)); i++ {
		if !titleWordEqual(sharedA[i], sharedB[i]) {
			sameOrder = false
		}
	}
	n := max(len(ua), len(ub))
	short, long := ua, ub
	if len(short) > len(long) {
		short, long = long, short
	}
	exactShared := 0
	for _, word := range short {
		for _, other := range long {
			if word == other {
				exactShared++
				break
			}
		}
	}
	// The panel titles hold no stop word and repeat no word; any other title
	// keeps the shared scoring below.
	plain := len(wa) == len(ua) && len(wb) == len(ub)
	// subset is the word-order rules' reading of an exact subset; the panel
	// extensions (exactSubset) also need plain titles.
	subset := exactShared == len(short)
	exactSubset := plain && subset
	if exactSubset && k == len(ua) && len(ub) == n && (n == 5 || n == 6) && (k == 1 || (k == 2 && !sameOrder)) {
		// NATIVE (duplicate_fuzzy_panels): a candidate whose content words
		// all occur in a longer existing title finds it from the short side
		// only: one word in five at 88, two words out of order in five at 90
		// and in six at 89. One word in six matches neither way.
		if confidence, ok := titleShortSide[[2]int{k, n}]; ok {
			return true, confidence
		}
	}
	if n >= 5 && (k == 1 || (k == 2 && !sameOrder)) {
		return false, 0
	}
	confidence, ok := titleConfidence[[2]int{min(k, n-1), min(n, 5)}]
	if n == 6 && ((exactSubset && len(short) < len(long)) || (len(short) == 3 && k == 2 && exactShared == 2 && sameOrder && plain)) {
		// NATIVE (duplicate_fuzzy_panels): six content words.
		confidence, ok = titleConfidenceSix[min(k, n-1)]
	}
	if !ok {
		confidence = 88
	}
	if sameOrder && n == 2 && k == 1 && confidence == 93 && len(wa) > len(ca) && titleAllIn(wa, wb) {
		// NATIVE (fuzzcap-C9 c9t4, fuzzcap-B3 tt2): a candidate with a stop
		// word whose words are all in the existing title: "Zorp Of" finds
		// "Zorp of Quill" and "Director IT" finds "Director - IT" at 94.
		confidence = 94
	}
	if !sameOrder {
		if subset && len(short) == k && len(short) < len(long) && k >= 2 && titleReversed(sharedA, sharedB) {
			// NATIVE (fuzzcap-C9 c9t2d, c9t3c, c9t3d): all words of the
			// shorter title in the longer one, in the reverse order (a word
			// moved to the other end keeps the general penalty). The penalty
			// depends on the direction and the two lengths.
			pen, known := titleReorderPenalty(len(ua) < len(ub), k, len(long))
			if !known {
				pen = 3
			}
			if pen < 0 {
				return false, 0
			}
			return true, confidence - float64(pen)
		}
		switch {
		case n == 6 && exactSubset && len(short) < len(long) && k == 5:
			confidence -= 2
		case k == 2 && n == 4 && !subset:
			// NATIVE (fuzzcap-C9 c9t2d): two shared words out of order in a
			// title of four.
			confidence -= 5
		default:
			confidence -= 3
		}
	}
	return true, confidence
}

func titleReversed(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !titleWordEqual(a[i], b[len(b)-1-i]) {
			return false
		}
	}
	return true
}

func titleAllIn(words, other []string) bool {
	for _, w := range words {
		if !slices.Contains(other, w) {
			return false
		}
	}
	return len(other) > len(words)
}

// titleReorderPenalty is the confidence a title loses when all its content
// words (candidate shorter) or all the words of the existing title
// (candidate longer) occur in the other title in a different order, by shared
// and longer word count. A negative penalty is no match.
func titleReorderPenalty(candidateShorter bool, k, long int) (int, bool) {
	type key struct{ k, long int }
	shorter := map[key]int{{2, 3}: 1, {3, 4}: 2, {3, 5}: 1, {3, 6}: 2, {4, 6}: 1, {5, 6}: 2}
	longer := map[key]int{{2, 3}: 4, {3, 4}: 4, {3, 5}: 5, {3, 6}: -1, {4, 6}: 4, {5, 6}: 2}
	table := longer
	if candidateShorter {
		table = shorter
	}
	p, ok := table[key{k, long}]
	return p, ok
}

// fuzzyItemScore converts a method's single-item confidence into the item
// score that rule confidences average: the Title rows were captured with an
// exact Last Name item, so their confidence is already the two-item average.
func fuzzyItemScore(method string, confidence float64) float64 {
	if strings.EqualFold(strings.TrimSpace(method), "Title") {
		return math.Max(0, 2*confidence-100)
	}
	return confidence
}

// companySlashKeys reports whether two names share the fitted match key when
// a slash joins the words on either side of it. NATIVE company_name_fit/org.tsv
// R1_dir and R1_qf2: "Orbel Trade/West" matches neither "Orbel Trade/East",
// "Orbel Trade West" nor "Orbel Trade - West", and "Orbel Trade/Midwest"
// matches "Orbel Trade/Mideast"; D1_qf: "Zorvex Freight/West" matches all
// three of its forms. No captured match with a slash has unequal joined keys.
func companySlashKeys(a, b string) bool {
	if strings.IndexByte(a, '/') < 0 && strings.IndexByte(b, '/') < 0 {
		return true
	}
	same, _, _ := companyFitKeys(strings.ReplaceAll(a, "/", ""), strings.ReplaceAll(b, "/", ""))
	return same
}

// companyKeyPrefix reports that the first words agree in their first six
// characters, the part the match key keeps. NATIVE company_name_fit/org.tsv
// R1_dbl: "Vexlett Packard" matches "Vexlet Packard" and "Vexlettt Packard";
// "Vexllet Packard", "Vvexlet Packard", "Qell Systems" and "Qel Systems" do
// not match their single-letter forms.
func companyKeyPrefix(a, b string) bool {
	for i := 0; i < 6; i++ {
		if i >= len(a) || i >= len(b) {
			return false
		}
		x, y := a[i], b[i]
		if x >= 'A' && x <= 'Z' {
			x += 'a' - 'A'
		}
		if y >= 'A' && y <= 'Z' {
			y += 'a' - 'A'
		}
		if x != y || x == ' ' || x == '-' {
			return false
		}
	}
	return true
}
