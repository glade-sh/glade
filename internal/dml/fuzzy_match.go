package dml

import (
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode"

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

// FuzzyScore scores two non-blank values under the method. The score is the
// match confidence a single-item rule reports for the pair (0 to 100); a
// matched pair always scores at least the method's threshold. matchBlanks is
// the item's blank handling, which the Street method applies to a blank
// street-number section.
func FuzzyScore(method, left, right string, matchBlanks bool) (bool, float64) {
	scorer, ok := fuzzyScorers[strings.ToLower(strings.TrimSpace(method))]
	if !ok {
		return false, 0
	}
	return scorer(strings.TrimSpace(left), strings.TrimSpace(right), matchBlanks)
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

// fuzzyFirstName: a is the candidate, b the existing record. A Name Variant
// entry of the candidate scores 100; the dictionary is directional ("Rosa"
// finds "Rose", "Rose" does not find "Rosa"). Otherwise the two names are
// compared only when their match keys agree, the first two characters of the
// Double Metaphone code ("Vorkel" and "Volkel" are never compared, "Vorkel"
// and "Vorpel" are), and match when Jaro-Winkler reaches the threshold, with
// confidence (100 + score) / 2. A dictionary name whose key comes from its
// alias and not from its own spelling ("Kate" is keyed as "Katherine", so it
// matches neither "Katie" nor "Katte") never matches by Jaro-Winkler.
func fuzzyFirstName(a, b string, _ bool) (bool, float64) {
	x, y := foldText(a), foldText(b)
	if x == y || nameVariantOf(x, y) {
		return true, 100
	}
	if kx, ky := firstNameKey(x), firstNameKey(y); (kx == "" || kx != ky || nameKeptApart[orderedPair(x, y)]) && !firstNameSameEntry(x, y) {
		return false, 0
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
	return doubleMetaphone(name, firstNameKeyLength)
}

// firstNameSameEntry reports a name and the same name followed by a second
// word or by punctuation. The dictionary lookup reads the first word without
// punctuation, so the two share a match key whatever that key is: "Billy Bob"
// finds "Billy" and "Jane." finds "Jane" by Jaro-Winkler although "Billy" and
// "Jane" are alias-keyed.
func firstNameSameEntry(x, y string) bool {
	head := func(s string) string {
		if i := strings.IndexByte(s, ' '); i > 0 {
			s = s[:i]
		}
		return strings.TrimFunc(s, unicode.IsPunct)
	}
	return head(x) == y || head(y) == x
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

// fuzzyLastName: the match key is the Double Metaphone code of the
// accent-folded name, four characters at most ("Frost" and "Frosr" are never
// compared, "Martinez" and "Martinpz" are). Two names that sound alike score
// 100; the platform decides that with Metaphone 3, which Glade does not
// carry, so lastNameSoundsAlike accepts only the spelling changes the native
// rows show to be code-neutral. Otherwise Keyboard Distance decides at 90
// with confidence (100 + score) / 2.
func fuzzyLastName(a, b string, _ bool) (bool, float64) {
	x, y := foldText(a), foldText(b)
	if x == y {
		return true, 100
	}
	if doubleMetaphone(stripAccents(x), lastNameKeyLength) != doubleMetaphone(stripAccents(y), lastNameKeyLength) {
		return false, 0
	}
	x, y = strings.ReplaceAll(x, "'", ""), strings.ReplaceAll(y, "'", "")
	if x == y || (lastNameSoundsAlike(x, y) && !lastNameKeptApart[orderedPair(x, y)]) {
		return true, 100
	}
	score, ok := keyboardSimilarity(x, y)
	return ok && score >= 90, averageWith100(score)
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

func isNameVowel(r rune) bool {
	if strings.ContainsRune("aeiouy", r) {
		return true
	}
	// An accented vowel, or a letter such as "ø" with no ASCII base.
	return r > unicode.MaxASCII && unicode.IsLetter(r) && r != 'ñ' && r != 'ç' && r != 'ß'
}

// nameRuns splits a folded name into alternating runs of vowels and of other
// characters.
func nameRuns(s string) []string {
	var runs []string
	var cur []rune
	vowel := false
	for i, r := range s {
		v := isNameVowel(r)
		if i > 0 && v != vowel && len(cur) > 0 {
			runs = append(runs, string(cur))
			cur = cur[:0]
		}
		vowel = v
		cur = append(cur, r)
	}
	if len(cur) > 0 {
		runs = append(runs, string(cur))
	}
	return runs
}

// lastNameShape renders a name for the sounds-alike comparison. With an
// empty edit, every vowel run that does not touch a context-sensitive
// consonant (c, g, h, j, x, or a t or s before it) and is not the first
// letter becomes a wildcard, so "Taylor", "Tayler" and "Tailor" share a
// shape. With an edit, vowels stay literal and one consonant respelling is
// folded: 'd' a doubled consonant, 'k' "ck" and "q" as "k", 'f' "ph" as "f",
// 'z' "z" as "s", 'h' an "h" outside a digraph, 'w' a "w" after the first
// letter, 'e' a final "e" after a consonant.
func lastNameShape(s string, edit byte, freeVowels bool) string {
	runs := nameRuns(strings.ReplaceAll(s, "ñ", "n"))
	if len(runs) > 1 && strings.HasPrefix(runs[0], "y") {
		// An initial "y" before a vowel is a consonant ("Young", "Yung").
		runs = append([]string{"y", runs[0][1:]}, runs[1:]...)
		if runs[1] == "" {
			runs = append(runs[:1], runs[2:]...)
		}
	}
	if edit == 'e' && len(runs) >= 3 && runs[len(runs)-1] == "e" {
		runs = runs[:len(runs)-1]
	}
	out := make([]string, 0, len(runs))
	for i, run := range runs {
		first := []rune(run)[0]
		if isNameVowel(first) && !(i == 0 && run == "y") {
			prev, next := "", ""
			if i > 0 {
				prev = runs[i-1]
			}
			if i+1 < len(runs) {
				next = runs[i+1]
			}
			sensitive := i == 0 || lastNameSensitive(prev) || lastNameSensitive(next) ||
				((strings.HasSuffix(prev, "t") || strings.HasSuffix(prev, "s")) && len([]rune(run)) > 1 && first == 'i')
			if !freeVowels || sensitive {
				out = append(out, run)
			} else {
				out = append(out, "*")
			}
			continue
		}
		r := []rune(run)
		switch edit {
		case 'd':
			var folded []rune
			for _, c := range r {
				if len(folded) == 0 || folded[len(folded)-1] != c {
					folded = append(folded, c)
				}
			}
			r = folded
		case 'k':
			// "ck" as "k", but not in "Mc" ("McKnight" is not "Mknight").
			if !(i == 0 && strings.HasPrefix(string(r), "mc")) {
				r = []rune(strings.ReplaceAll(string(r), "ck", "k"))
			}
			r = []rune(strings.ReplaceAll(string(r), "q", "k"))
		case 'f':
			r = []rune(strings.ReplaceAll(string(r), "ph", "f"))
		case 'z':
			r = []rune(strings.ReplaceAll(string(r), "z", "s"))
		case 't':
			r = []rune(strings.ReplaceAll(strings.ReplaceAll(string(r), "tch", "ch"), "sch", "sh"))
		case 'h', 'w':
			var kept []rune
			for j, c := range r {
				drop := false
				if !(i == 0 && j == 0) {
					if edit == 'w' && c == 'w' {
						drop = true
					}
					if edit == 'h' && c == 'h' && !(j > 0 && strings.ContainsRune("cstpg", r[j-1])) {
						drop = true
					}
				}
				if !drop {
					kept = append(kept, c)
				}
			}
			r = kept
		}
		if len(r) == 0 {
			out = append(out, "-")
		} else {
			out = append(out, string(r))
		}
	}
	return strings.Join(out, "|")
}

// lastNameSensitive reports a consonant run whose sound depends on the
// vowels next to it: one with c, g, j or x ("Lach" does not find "Leach").
func lastNameSensitive(run string) bool {
	return strings.ContainsAny(run, "cgxj")
}

var (
	lastNameSilentL    = regexp.MustCompile(`[ao]l+[kmfv]|ould`)
	lastNameDoubleRisk = regexp.MustCompile(`[ao]l+[kmfv]|l+n$`)
	lastNameSoftBefore = regexp.MustCompile(`[cg][eiy]+$`)
	lastNameWeakVowel  = regexp.MustCompile(`[eiuy]`)
)

// lastNameDoubleSafe rejects the doubled consonants the rows show Metaphone 3
// reads differently from the single letter: next to a silent L ("Walker",
// "Wallker"), after a soft c or g ("Girard", "Girrard"), before or as one of
// c, g, h, j, x ("Bethea", "Betthea"; "Bence", "Bennce").
func lastNameDoubleSafe(s string) bool {
	if lastNameDoubleRisk.MatchString(s) {
		return false
	}
	r := []rune(s)
	for i := 1; i < len(r); i++ {
		if r[i] != r[i-1] || isNameVowel(r[i]) {
			continue
		}
		start := i - 1
		for start > 0 && r[start-1] == r[i] {
			start--
		}
		end := i + 1
		for end < len(r) && r[end] == r[i] {
			end++
		}
		if strings.ContainsRune("cghxj", r[i]) || lastNameSoftBefore.MatchString(string(r[:start])) || (end < len(r) && strings.ContainsRune("cghxj", r[end])) {
			return false
		}
		i = end - 1
	}
	return true
}

// lastNameSoundsAlike reports two names that differ only by spelling changes
// the native rows score 100 (equal Metaphone 3 codes): any change inside
// vowel runs away from a context-sensitive consonant, or one kind of
// consonant respelling with every vowel unchanged. A dropped vowel is not
// accepted: natively "Smith" finds "Smth" but "Cantu" does not find "Cntu".
func lastNameSoundsAlike(x, y string) bool {
	if lastNameShape(x, 0, true) == lastNameShape(y, 0, true) {
		if (lastNameSilentL.MatchString(x) || lastNameSilentL.MatchString(y)) && lastNameWeakVowel.ReplaceAllString(x, "*") != lastNameWeakVowel.ReplaceAllString(y, "*") {
			return false
		}
		return true
	}
	if lastNameMac.ReplaceAllString(x, "mc$1") == lastNameMac.ReplaceAllString(y, "mc$1") {
		return true
	}
	for _, edit := range []byte("zhwdekft") {
		if lastNameShape(x, edit, false) == lastNameShape(y, edit, false) {
			return edit != 'd' || (lastNameDoubleSafe(x) && lastNameDoubleSafe(y))
		}
		// A final "e", a silent "h" or "w" and "z" for "s" also combine with
		// vowel changes ("Smithe" and "Smyth", "Brown" and "Braun"); the
		// other respellings do not ("Lammb" does not find "Lemb").
		if strings.ContainsRune("ehwz", rune(edit)) && lastNameShape(x, edit, true) == lastNameShape(y, edit, true) {
			return !lastNameSilentL.MatchString(x) && !lastNameSilentL.MatchString(y)
		}
	}
	return false
}

var lastNameMac = regexp.MustCompile(`^mac([^aeiouy])`)

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
	x, y := streetClean(a), streetClean(b)
	if x == y {
		return true, 100
	}
	if confidence, ok := streetAbbreviationPair(x, y); ok {
		return true, confidence
	}
	pa, pb := parseStreet(a), parseStreet(b)
	sections := streetSections(pa, pb, matchBlanks)
	confidence := (sections + editSimilarity(x, y)) / 2
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

func fuzzyCompany(a, b string, _ bool) (bool, float64) {
	x, y := foldText(a), foldText(b)
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
	if companyKey(wa) != companyKey(wb) {
		return false, 0
	}
	confidence := 100.0
	same := true
	for i := 0; i < min(len(wa), len(wb)); i++ {
		ok, c := companyWordMatch(wa[i], wb[i])
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
	if n >= 5 && (k == 1 || (k == 2 && !sameOrder)) {
		return false, 0
	}
	confidence, ok := titleConfidence[[2]int{min(k, n-1), min(n, 5)}]
	if !ok {
		confidence = 88
	}
	if !sameOrder {
		confidence -= 3
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
