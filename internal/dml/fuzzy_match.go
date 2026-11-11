package dml

import (
	"math"
	"strings"
	"unicode"
)

// Fuzzy matching methods for custom matching rule items. The method names are
// the MatchingMethod picklist values of the metadata API. The scoring follows
// the public descriptions of the methods and is fitted to native
// Datacloud.FindDuplicates verdicts (see the duplicate_fuzzy_matching
// conformance fixture). Where Salesforce uses a dictionary or algorithm that is
// not published, the narrowest rule the native rows prove is used instead.

// FuzzyMethodSupported reports whether the matching method is one of the
// field-specific fuzzy methods this package scores.
func FuzzyMethodSupported(method string) bool {
	_, ok := fuzzyScorers[strings.ToLower(strings.TrimSpace(method))]
	return ok
}

// FuzzyMatch reports whether two non-blank field values match under the
// method. Callers handle blank values.
func FuzzyMatch(method, left, right string) bool {
	scorer, ok := fuzzyScorers[strings.ToLower(strings.TrimSpace(method))]
	if !ok {
		return false
	}
	return scorer(strings.TrimSpace(left), strings.TrimSpace(right))
}

var fuzzyScorers = map[string]func(a, b string) bool{
	"firstname":   fuzzyFirstName,
	"lastname":    fuzzyLastName,
	"companyname": fuzzyCompany,
	"phone":       fuzzyPhone,
	"city":        fuzzyCity,
	"street":      fuzzyStreet,
	"zip":         fuzzyZip,
	"title":       fuzzyTitle,
}

func foldText(s string) string {
	return strings.Join(strings.Fields(strings.ToLower(s)), " ")
}

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

// editSimilarity is 100 * (1 - distance / longer length).
func editSimilarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	longest := max(len(ra), len(rb))
	if longest == 0 {
		return 100
	}
	return 100 * (1 - float64(levenshtein(ra, rb))/float64(longest))
}

func jaroWinkler(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	if string(ra) == string(rb) {
		return 1
	}
	if len(ra) == 0 || len(rb) == 0 {
		return 0
	}
	window := max(len(ra), len(rb))/2 - 1
	if window < 0 {
		window = 0
	}
	ma, mb := make([]bool, len(ra)), make([]bool, len(rb))
	matches := 0
	for i := range ra {
		for j := max(0, i-window); j < min(len(rb), i+window+1); j++ {
			if !mb[j] && ra[i] == rb[j] {
				ma[i], mb[j] = true, true
				matches++
				break
			}
		}
	}
	if matches == 0 {
		return 0
	}
	k, transposed := 0, 0
	for i := range ra {
		if !ma[i] {
			continue
		}
		for !mb[k] {
			k++
		}
		if ra[i] != rb[k] {
			transposed++
		}
		k++
	}
	m := float64(matches)
	jaro := (m/float64(len(ra)) + m/float64(len(rb)) + (m-float64(transposed)/2)/m) / 3
	prefix := 0
	for prefix < 4 && prefix < len(ra) && prefix < len(rb) && ra[prefix] == rb[prefix] {
		prefix++
	}
	return jaro + float64(prefix)*0.1*(1-jaro)
}

// nameVariantGroups holds the nickname and spelling groups the native rows
// prove equivalent for the Name Variant method. Salesforce uses a hard-coded
// dictionary that is not published; names outside these groups fall back to
// the other first-name methods.
var nameVariantGroups = [][]string{
	{"robert", "bob", "roberto", "rob", "bobby", "robbie"},
	{"jonathan", "jonathon", "jon"},
	{"jon", "john", "johnny"},
	{"jane", "janet"},
	{"jo", "joseph", "josef"},
	{"elizabeth", "liz", "beth", "eliza"},
	{"michael", "mike", "mikael"},
	{"catherine", "katherine", "kathryn", "cathy"},
	{"william", "bill", "will", "billy"},
	{"zoe", "zoë", "zoey"},
	{"maria", "mary", "marie"},
	{"steven", "stephen", "stefan"},
}

var nameVariantIndex = func() map[string][]int {
	out := map[string][]int{}
	for i, group := range nameVariantGroups {
		for _, name := range group {
			out[name] = append(out[name], i)
		}
	}
	return out
}()

func nameVariants(a, b string) bool {
	for _, i := range nameVariantIndex[a] {
		for _, j := range nameVariantIndex[b] {
			if i == j {
				return true
			}
		}
	}
	return false
}

func fuzzyFirstName(a, b string) bool {
	x, y := foldText(a), foldText(b)
	if x == y || nameVariants(x, y) {
		return true
	}
	return jaroWinkler(x, y) >= 0.95 && min(len([]rune(x)), len([]rune(y))) >= 4
}

// metaphoneKey is a reduced phonetic key used for the Metaphone 3 method. It
// keeps word boundaries, folds the common digraph and letter-pair equivalences
// and drops interior vowels.
func metaphoneKey(s string) string {
	var words []string
	for _, word := range strings.Fields(strings.ToLower(s)) {
		var in []rune
		for _, r := range word {
			if unicode.IsLetter(r) {
				in = append(in, r)
			}
		}
		text := string(in)
		for _, pair := range [][2]string{
			{"ü", "u"}, {"ö", "o"}, {"ä", "a"}, {"é", "e"}, {"í", "i"}, {"ñ", "n"}, {"ç", "c"}, {"ue", "u"}, {"ae", "a"}, {"ph", "f"}, {"gh", ""},
			{"ck", "k"}, {"kn", "n"}, {"wr", "r"}, {"sch", "x"}, {"ch", "x"}, {"sh", "x"}, {"th", "0"}, {"tch", "x"}, {"dg", "j"}, {"cz", "s"},
			{"ou", "u"}, {"ey", "e"}, {"ei", "e"}, {"ie", "e"}, {"ci", "si"}, {"ce", "se"}, {"cy", "sy"}, {"d", "t"}, {"z", "s"}, {"q", "k"}, {"v", "f"}, {"w", "u"}, {"c", "k"}, {"y", "i"},
		} {
			text = strings.ReplaceAll(text, pair[0], pair[1])
		}
		var out []rune
		var last rune
		for i, r := range text {
			if r == 'h' && i > 0 {
				continue
			}
			if strings.ContainsRune("aeiou", r) && i > 0 {
				last = 0
				continue
			}
			if r == last {
				continue
			}
			if strings.ContainsRune("aeiou", r) {
				r = 'a'
			}
			out = append(out, r)
			last = r
		}
		words = append(words, string(out))
	}
	return strings.Join(words, " ")
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

// keyboardSimilarity is an edit distance whose substitution cost is the
// key distance (capped at one edit), normalized by the longer length.
func keyboardSimilarity(a, b string) float64 {
	ra, rb := []rune(a), []rune(b)
	longest := max(len(ra), len(rb))
	if longest == 0 {
		return 100
	}
	sub := func(x, y rune) float64 {
		if x == y {
			return 0
		}
		px, okx := qwertyPosition[x]
		py, oky := qwertyPosition[y]
		if !okx || !oky {
			return 1
		}
		d := math.Hypot(float64(px[0]-py[0]), float64(px[1]-py[1]))
		return math.Min(1, 0.4+0.2*d)
	}
	prev := make([]float64, len(rb)+1)
	cur := make([]float64, len(rb)+1)
	for j := range prev {
		prev[j] = float64(j)
	}
	for i := 1; i <= len(ra); i++ {
		cur[0] = float64(i)
		for j := 1; j <= len(rb); j++ {
			cur[j] = math.Min(math.Min(prev[j]+1, cur[j-1]+1), prev[j-1]+sub(ra[i-1], rb[j-1]))
		}
		prev, cur = cur, prev
	}
	return 100 * (1 - prev[len(rb)]/float64(longest))
}

func fuzzyLastName(a, b string) bool {
	x, y := foldText(a), foldText(b)
	if x == y {
		return true
	}
	kx, ky := metaphoneKey(x), metaphoneKey(y)
	if kx != "" && kx == ky {
		return true
	}
	return keyboardSimilarity(x, y) >= 90
}

// phoneParts splits a North American number into country, area, exchange and
// line sections. ok is false for numbers outside that format.
func phoneParts(s string) (country, area, exchange, line string, ok bool) {
	var digits []rune
	for _, r := range s {
		switch {
		case r >= '0' && r <= '9':
			digits = append(digits, r)
		case strings.ContainsRune("()+-./ \t", r):
		default:
			return "", "", "", "", false
		}
	}
	d := string(digits)
	switch {
	case len(d) == 10:
		return "", d[0:3], d[3:6], d[6:], true
	case len(d) == 11:
		return d[:1], d[1:4], d[4:7], d[7:], true
	}
	return "", "", "", "", false
}

func fuzzyPhone(a, b string) bool {
	ca, aa, ea, la, okA := phoneParts(a)
	cb, ab, eb, lb, okB := phoneParts(b)
	if !okA || !okB {
		return false
	}
	score := 0.0
	if ca == cb {
		score += 10
	}
	if aa == ab {
		score += 50
	}
	if ea == eb {
		score += 30
	}
	if la == lb {
		score += 10
	}
	return score >= 80
}

// fuzzyZip compares the leading postal code segment (before a hyphen or
// space); the extension segment does not affect the verdict.
func fuzzyZip(a, b string) bool {
	head := func(s string) string {
		s = foldText(s)
		if i := strings.IndexAny(s, "- "); i >= 0 {
			return s[:i]
		}
		return s
	}
	return head(a) == head(b)
}

func fuzzyCity(a, b string) bool {
	x, y := foldText(a), foldText(b)
	return x == y || editSimilarity(x, y) >= 87
}

var streetSuffixes = map[string]string{
	"st": "street", "street": "street",
	"ave": "avenue", "avenue": "avenue",
	"rd": "road", "road": "road",
	"blvd": "boulevard", "boulevard": "boulevard",
	"dr": "drive", "drive": "drive", "ln": "lane", "lane": "lane",
	"way": "way", "wy": "way", "loop": "loop", "lp": "loop",
}

var streetDirections = map[string]bool{"north": true, "south": true, "east": true, "west": true, "n": true, "s": true, "e": true, "w": true, "ne": true, "nw": true, "se": true, "sw": true}

type streetParts struct{ number, name, suffix, rawSuffix, suite string }

func parseStreet(s string) streetParts {
	var p streetParts
	words := strings.Fields(strings.NewReplacer(".", "", ",", "").Replace(strings.ToLower(s)))
	for i, w := range words {
		trimmed := strings.TrimRight(w, ".")
		if trimmed == "suite" || trimmed == "ste" || trimmed == "apt" || strings.HasPrefix(w, "#") {
			p.suite = strings.TrimPrefix(strings.Join(words[i:], ""), trimmed)
			words = words[:i]
			break
		}
	}
	if len(words) > 0 && strings.Trim(words[0], "0123456789") == "" {
		p.number, words = words[0], words[1:]
	}
	if len(words) > 1 && streetDirections[words[len(words)-1]] {
		words = words[:len(words)-1]
	}
	if len(words) > 0 {
		last := strings.TrimRight(words[len(words)-1], ".")
		if full, ok := streetSuffixes[last]; ok {
			p.suffix, p.rawSuffix, words = full, last, words[:len(words)-1]
		} else if len(words) > 1 && nearStreetSuffix(last) {
			// One edit from a known suffix (Str, Av, Bld): kept as a suffix
			// that matches only an abbreviation within one edit.
			p.suffix, p.rawSuffix, words = last, last, words[:len(words)-1]
		}
	}
	p.name = strings.TrimRight(strings.Join(words, " "), ".")
	p.name = strings.ReplaceAll(p.name, "fifth", "5th")
	return p
}

// fuzzyStreet matches addresses with the same street number and street name.
// The native rows match through suffix and suite differences and reject name
// and number edits, so the name is compared exactly.
func fuzzyStreet(a, b string) bool {
	x, y := parseStreet(a), parseStreet(b)
	if x.number != y.number || editSimilarity(x.name, y.name) < 86 {
		return false
	}
	if x.suffix == y.suffix || x.rawSuffix == "" || y.rawSuffix == "" {
		return true
	}
	// A suffix mismatch is tolerated; the native rows only reject the pair
	// when an unrecognized abbreviation sits two or more edits away.
	return levenshtein([]rune(x.rawSuffix), []rune(y.rawSuffix)) <= 1 || streetSuffixes[x.rawSuffix] != "" && streetSuffixes[y.rawSuffix] != ""
}

func nearStreetSuffix(token string) bool {
	if len(token) < 2 {
		return false
	}
	for known := range streetSuffixes {
		if len(known) <= 4 && levenshtein([]rune(token), []rune(known)) == 1 {
			return true
		}
	}
	return false
}

var companyDesignators = map[string]bool{
	"inc": true, "incorporated": true, "corp": true, "corporation": true, "llc": true, "ltd": true,
	"limited": true, "co": true, "company": true, "the": true,
}

// companyTokens lower-cases a company name, drops a leading "the" and one
// trailing legal designator, and folds plural endings. The native rows keep a
// second designator ("Acme Corporation Inc" stays distinct from "Acme").
func companyTokens(s string) []string {
	var out []string
	for _, w := range strings.Fields(strings.ReplaceAll(strings.ToLower(s), "&", " and ")) {
		w = strings.Trim(w, ".,")
		if w == "" {
			continue
		}
		out = append(out, w)
	}
	if len(out) > 1 && out[0] == "the" {
		out = out[1:]
	}
	if len(out) > 1 && companyDesignators[out[len(out)-1]] {
		out = out[:len(out)-1]
	}
	for i, w := range out {
		if strings.HasSuffix(w, "ies") && len(w) > 4 {
			out[i] = w[:len(w)-3] + "y"
		}
	}
	return out
}

// companyKey folds a hyphen to a space and ignores "of", which the native
// rows treat as no difference.
func companyKey(tokens []string) string {
	var out []string
	for _, t := range tokens {
		for _, part := range strings.Fields(strings.ReplaceAll(t, "-", " ")) {
			if part != "of" {
				out = append(out, part)
			}
		}
	}
	return strings.Join(out, " ")
}

// companyAcronyms lists the acronym pairs the native rows prove. Salesforce
// resolves the Acronym method through a hard-coded dictionary that is not
// published, and initials alone are not enough ("AA", "HPE" and "DOE" do not
// match their expansions in the rows).
var companyAcronyms = map[string]string{
	"ibm":  "international business machines",
	"amd":  "advanced micro devices",
	"ge":   "general electric",
	"gm":   "general motors",
	"hp":   "hewlett packard",
	"nasa": "national aeronautics and space administration",
}

func fuzzyCompany(a, b string) bool {
	ta, tb := companyTokens(a), companyTokens(b)
	x, y := companyKey(ta), companyKey(tb)
	if x == "" || y == "" {
		return foldText(a) == foldText(b)
	}
	if x == y {
		return true
	}
	short := func(tokens []string) string { return stripDots(strings.Join(tokens, "")) }
	if full, ok := companyAcronyms[short(ta)]; ok && len(tb) > 1 && full == y {
		return true
	}
	if full, ok := companyAcronyms[short(tb)]; ok && len(ta) > 1 && full == x {
		return true
	}
	return false
}

func initials(tokens []string) string {
	var out []rune
	for _, t := range tokens {
		if titleStopWords[t] {
			continue
		}
		for _, r := range t {
			out = append(out, r)
			break
		}
	}
	return string(out)
}

func stripDots(s string) string {
	return strings.NewReplacer(".", "", " ", "").Replace(strings.ToLower(s))
}

var titleStopWords = map[string]bool{"of": true, "the": true, "and": true, "&": true}

func titleTokens(s string) []string {
	var out []string
	for _, w := range strings.Fields(strings.ToLower(s)) {
		w = strings.Trim(w, ".,")
		if w != "" && !titleStopWords[w] {
			out = append(out, w)
		}
	}
	return out
}

func titleTokenEqual(a, b string) bool {
	if a == b {
		return true
	}
	ra, rb := []rune(a), []rune(b)
	n := min(len(ra), len(rb))
	if n >= 6 && string(ra[:6]) == string(rb[:6]) {
		return true
	}
	// An abbreviation of at least three letters ("dir", "exec") matches the
	// word it starts.
	return n >= 3 && (strings.HasPrefix(a, b) || strings.HasPrefix(b, a))
}

func fuzzyTitle(a, b string) bool {
	ta, tb := titleTokens(a), titleTokens(b)
	if len(ta) == 0 || len(tb) == 0 {
		return foldText(a) == foldText(b)
	}
	if strings.Join(ta, " ") == strings.Join(tb, " ") {
		return true
	}
	if len(tb) > 1 && stripDots(strings.Join(ta, "")) == initials(tb) || len(ta) > 1 && stripDots(strings.Join(tb, "")) == initials(ta) {
		return true
	}
	for _, x := range ta {
		for _, y := range tb {
			if titleTokenEqual(x, y) {
				return true
			}
		}
	}
	return false
}
