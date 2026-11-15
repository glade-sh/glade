package dml

import (
	"math"
	"regexp"
	"sort"
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
// matches neither "Katie" nor "Katte") never matches by Jaro-Winkler. Then the
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
		if kx, ky := firstNameKey(wx), firstNameKey(wy); kx == "" || kx != ky || nameKeptApart[orderedPair(fx, fy)] {
			return false, 0
		}
	}
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
// Jaro-Winkler, and "Yancey" finds "Yancey R" only by Jaro-Winkler.
func firstNameInitials(initials, name string) bool {
	word := firstNameWord(initials)
	letter, size := utf8.DecodeRuneInString(word)
	if size == 0 || size != len(word) || !unicode.IsLetter(letter) || strings.IndexByte(name, ' ') >= 0 {
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
	return lastNameKeyboard(x, y)
}

var lastNameApostrophes = strings.NewReplacer("'", "", "\u2019", "")

// lastNameKeyboard is the Keyboard Distance step of the Last Name method on
// two folded names whose apostrophes are removed.
func lastNameKeyboard(x, y string) (bool, float64) {
	if len(x) == 8 && standardIsTerminalSwap(x, y) {
		a, b := qwertyPosition[rune(x[6])], qwertyPosition[rune(x[7])]
		// Only the captured five-column geometry overrides ordinary membership.
		if abs(a[0]-b[0]) <= 1 && abs(a[1]-b[1]) == 5 {
			ordinary, ok := keyboardSimilarity(x, y)
			return standardTerminalSwap(x, averageWith100(ordinary), ok && ordinary >= 90)
		}
	}
	score, ok := keyboardSimilarity(x, y)
	return ok && score >= 90, averageWith100(score)
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

// ---- ZIP: the segment before the first hyphen or space decides ----

func fuzzyZip(a, b string, _ bool) (bool, float64) {
	head := func(s string) string {
		s = foldText(s)
		if i := strings.IndexAny(s, "- "); i >= 0 {
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

func fuzzyCity(a, b string, _ bool) (bool, float64) {
	x, y := foldText(a), foldText(b)
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

var streetDirections = map[string]bool{"n": true, "s": true, "e": true, "w": true, "ne": true, "nw": true, "se": true, "sw": true, "north": true, "south": true, "east": true, "west": true}

var streetOrdinals = map[string]string{"first": "1st", "second": "2nd", "third": "3rd", "fourth": "4th", "fifth": "5th", "sixth": "6th", "seventh": "7th", "eighth": "8th", "ninth": "9th", "tenth": "10th"}

var digitsOnly = regexp.MustCompile(`^[0-9]+$`)

// streetSuiteDesignators are the words the rows drop before a suite value.
var streetSuiteDesignators = map[string]bool{"suite": true, "ste": true, "apt": true, "unit": true, "rm": true, "bldg": true}

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
func streetClean(s string) string {
	s = strings.NewReplacer(",", " ", ".", "", "#", " ").Replace(strings.ToLower(s))
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
func parseStreet(s string) streetParts {
	var p streetParts
	words := strings.Fields(streetClean(s))
	if len(words) > 0 && words[0][0] >= '0' && words[0][0] <= '9' {
		p.number, words = words[0], words[1:]
		if len(words) > 1 && len(words[0]) == 1 && isAlphaWord(words[0]) {
			if words[0] != "e" {
				p.number += " " + words[0]
			}
			words = words[1:]
		}
	}
	suffixAt := -1
	for i := 1; i < len(words); i++ {
		if _, ok := knownStreetSuffixes[words[i]]; ok {
			suffixAt = i
			break
		}
	}
	switch {
	case len(words) > 1 && streetPrefixSuffixes[words[0]]:
		p.suffix, words = words[0], words[1:]
	case suffixAt > 0:
		p.suffix = knownStreetSuffixes[words[suffixAt]]
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
		case section.name:
			den += section.weight
			num += section.weight * editSimilarity(section.x, section.y) / 100
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
	return fuzzyStreetWithCustom(a, b, matchBlanks, false)
}

func fuzzyStreetWithCustom(a, b string, matchBlanks, custom bool) (bool, float64) {
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

	if custom && sections >= 74 && sections < 80 && similarity >= 87.5-1e-9 {
		ia, ib := strings.IndexByte(a, ' '), strings.IndexByte(b, ' ')
		la, lb := strings.LastIndexByte(a, ' '), strings.LastIndexByte(b, ' ')
		if ia > 0 && ia == ib && a[:ia] == b[:ib] && la > ia && lb > ib && strings.EqualFold(a[:la], b[:lb]) {
			wx, wy := strings.Fields(x), strings.Fields(y)
			if len(wx) >= 3 && len(wx) == len(wy) && wx[0] == wy[0] && digitsOnly.MatchString(wx[0]) {
				last := len(wx) - 1
				if strings.Join(wx[1:last], " ") == strings.Join(wy[1:last], " ") && len(wx[1]) >= 2 {
					sx, sy := wx[last], wy[last]
					_, kx := knownStreetSuffixes[sx]
					_, ky := knownStreetSuffixes[sy]
					unknown := sx
					if kx {
						unknown = sy
					}
					if kx != ky && isAlphaWord(unknown) && !streetDirections[unknown] && !streetSuiteDesignators[unknown] && !streetPrefixSuffixes[unknown] {
						return true, confidence
					}
				}
			}
		}
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
	var words []string
	for _, token := range strings.Fields(strings.ReplaceAll(strings.ToLower(s), "-", " ")) {
		if token == "&" {
			words = append(words, "and")
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
	var out []string
	for _, w := range words {
		if !companyStopWords[w] {
			out = append(out, w)
		}
	}
	if len(out) == 0 {
		return words
	}
	return out
}

// companyKey is the match key: the first five characters of each of the first
// two words (help notation "Company (2,5)").
func companyKey(words []string) string {
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

func fuzzyCompany(a, b string, _ bool) (bool, float64) {
	x, y := foldText(a), foldText(b)
	if companyAlignedWords[strings.TrimSpace(x)] || companyAlignedWords[strings.TrimSpace(y)] {
		// NATIVE: a name that is only "Partner" matches nothing, itself included.
		return false, 0
	}
	if x == y {
		return true, 100
	}
	wa, wb := companyWords(a), companyWords(b)
	if len(wa) == 0 || len(wb) == 0 {
		return false, 0
	}
	ja, jb := strings.Join(wa, " "), strings.Join(wb, " ")
	if strings.Contains(a, "-") != strings.Contains(b, "-") {
		// A hyphenated name matches only the same words spelled with a space;
		// the rows score it 97 or 100 by direction and match neither its
		// acronym nor the same words plus one.
		return ja == jb, 97
	}
	if companyAcronyms[ja] == jb || companyAcronyms[jb] == ja {
		return true, 100
	}
	if len(wa) == 1 && len(wb) == 1 && companyKey(wa) == companyKey(wb) {
		if ok, c := companySingleWordMatch(wa[0], wb[0]); ok {
			return true, c
		}
	}
	if companyKey(wa) != companyKey(wb) {
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

// ---- Title: Acronym, Exact, Kullback-Leibler distance; threshold 50 ----

var titleStopWords = map[string]bool{"and": true, "of": true, "the": true}

func titleWords(s string) []string {
	var out []string
	for _, w := range strings.Fields(strings.ReplaceAll(strings.ToLower(s), "&", " and ")) {
		if w = strings.Trim(w, ".,"); w != "" {
			out = append(out, w)
		}
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
func titleWordEqual(a, b string) bool {
	if a == b {
		return true
	}
	if min(len(a), len(b)) < 2 {
		return false
	}
	if titleNoPrefix[a] || titleNoPrefix[b] {
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

func titleInitials(words []string) string {
	var b strings.Builder
	for _, w := range words {
		b.WriteString(w[:1])
	}
	return b.String()
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

func uniqueWords(words []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, w := range words {
		if !seen[w] {
			seen[w] = true
			out = append(out, w)
		}
	}
	return out
}

func sortedCopy(words []string) []string {
	out := append([]string(nil), words...)
	sort.Strings(out)
	return out
}

// fuzzyTitle returns the rule confidence directly (see titleConfidence); the
// caller converts it to the item score.
func fuzzyTitle(a, b string, _ bool) (bool, float64) {
	wa, wb := titleWords(a), titleWords(b)
	if len(wa) == 0 || len(wb) == 0 {
		return foldText(a) == foldText(b), 100
	}
	var ca, cb []string
	for _, w := range wa {
		if !titleStopWords[w] {
			ca = append(ca, w)
		}
	}
	for _, w := range wb {
		if !titleStopWords[w] {
			cb = append(cb, w)
		}
	}
	if strings.Join(ca, " ") == strings.Join(cb, " ") || strings.Join(sortedCopy(wa), " ") == strings.Join(sortedCopy(wb), " ") {
		return true, 100
	}
	if len(ca) == 1 && len(cb) == 1 {
		return false, 0
	}
	if (len(cb) > 1 && strings.Join(ca, "") == titleInitials(cb)) || (len(ca) > 1 && strings.Join(cb, "") == titleInitials(ca)) {
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
		return false, 0
	}
	if k == len(ua) && k == len(ub) && len(sharedB) == len(cb) {
		// The same content words in a different order: exact unless a stop
		// word made the sorted word lists differ.
		if len(wa) == len(ca) && len(wb) == len(cb) {
			return true, 100
		}
		return true, 93.5
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
	exactSubset := exactShared == len(short)
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
	if n == 6 && ((exactSubset && len(short) < len(long)) || (len(short) == 3 && k == 2 && exactShared == 2 && sameOrder)) {
		// NATIVE (duplicate_fuzzy_panels): six content words.
		confidence, ok = titleConfidenceSix[min(k, n-1)]
	}
	if !ok {
		confidence = 88
	}
	if !sameOrder {
		if n == 6 && exactSubset && len(short) < len(long) && k == 5 {
			confidence -= 2
		} else {
			confidence -= 3
		}
	}
	return true, confidence
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
