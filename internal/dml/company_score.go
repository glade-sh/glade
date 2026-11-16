package dml

import (
	"math"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

var companyExpansions = map[string]string{"intl": "international", "natl": "national", "univ": "university", "st": "saint", "mt": "mount", "assoc": "association", "dept": "department", "mfg": "manufacturing", "svcs": "services", "us": "unitedstates"}

// companyScoreWords normalizes words used by the directional scorer. The key
// deliberately has different punctuation handling and does not expand words.
func companyScoreWords(s string, key bool) []string {
	var b strings.Builder
	b.Grow(len(s) + 4)
	if isASCII(s) {
		for i := 0; i < len(s); i++ {
			switch c := s[i]; {
			case c >= 'a' && c <= 'z' || c >= '0' && c <= '9':
				b.WriteByte(c)
			case c >= 'A' && c <= 'Z':
				b.WriteByte(c + 'a' - 'A')
			case c == '&':
				b.WriteString(" and ")
			case c == '.':
			case !key && (c == '-' || c == '_'):
			default:
				b.WriteByte(' ')
			}
		}
		return companyScoreFields(b.String(), key)
	}
	for _, r := range strings.ToLower(s) {
		switch {
		case r == '&':
			b.WriteString(" and ")
		case r == '.':
		case !key && (r == '-' || r == '_'):
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
		default:
			b.WriteByte(' ')
		}
	}
	return companyScoreFields(b.String(), key)
}

// companyScoreFields is companyScoreWords after punctuation handling.
func companyScoreFields(s string, key bool) []string {
	w := strings.Fields(s)
	if !key {
		for i, v := range w {
			if x := companyExpansions[v]; x != "" {
				w[i] = x
			}
		}
	}
	for len(w) > 1 {
		n := len(w)
		if n > 2 && ((w[n-2] == "pvt" && w[n-1] == "ltd") || (w[n-2] == "private" && w[n-1] == "limited") || (w[n-2] == "s" && w[n-1] == "a") || (w[n-2] == "b" && w[n-1] == "v") || (w[n-2] == "incorporated" && w[n-1] == "company")) {
			w = w[:n-2]
			continue
		}
		if n > 3 && w[n-3] == "l" && w[n-2] == "l" && w[n-1] == "c" {
			w = w[:n-3]
			continue
		}
		if companyDesignators[w[n-1]] || w[n-1] == "partner" || w[n-1] == "sarl" || w[n-1] == "bv" {
			w = w[:n-1]
			continue
		}
		break
	}
	if len(w) > 1 && w[0] == "the" {
		w = w[1:]
	}
	stop := func(v string) bool { return v == "and" || v == "of" || v == "the" || v == "a" }
	n := 0
	for _, v := range w {
		if stop(v) {
			n++
		}
	}
	if n == 0 || n == len(w) {
		return w
	}
	out := make([]string, 0, len(w)-n)
	for _, v := range w {
		if !stop(v) {
			out = append(out, v)
		}
	}
	return out
}
func companyFitKey(s string) string {
	return companyFitKeyOf(companyScoreWords(s, true))
}

// companyFitKeyOf joins the first six characters of the first two words.
func companyFitKeyOf(w []string) string {
	var b strings.Builder
	for i, v := range w {
		if i == 2 {
			break
		}
		r := []rune(v)
		if len(r) > 6 {
			r = r[:6]
		}
		b.WriteString(string(r))
	}
	return b.String()
}
func companyNumber(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}
func companyGap(s string, candidate bool) float64 {
	if companyNumber(s) {
		v := .12 + .08*float64(len(s)-1)
		if candidate {
			v += .18
		}
		return v
	}
	if candidate {
		return .54
	}
	if len([]rune(s)) == 1 {
		return .24
	}
	return .36
}
func companyTokenValue(a, b string) float64 {
	if a == b || (len([]rune(a)) >= 2 && len([]rune(b)) >= 2 && (strings.HasPrefix(a, b) || strings.HasPrefix(b, a))) {
		return 1
	}
	if companyNumber(a) && companyNumber(b) {
		if strings.TrimLeft(a, "0") == strings.TrimLeft(b, "0") {
			return .79
		}
		common := 0
		for common < min(len(a), len(b)) && a[common] == b[common] {
			common++
		}
		p := .21*float64(min(len(a), len(b))-common) + .15*math.Abs(float64(len(a)-len(b)))
		return 1 - math.Max(.21, math.Min(.36, p))
	}
	sim := jaroWinkler(a, b)
	if sim >= .85 {
		return .88
	}
	if sim >= .7 {
		return .76
	}
	return .46
}
func companyTokenScore(a, b []string) float64 {
	if len(a) == 0 {
		if len(b) == 0 {
			return 100
		}
		return 0
	}
	if len(b) == 0 {
		return 0
	}
	prev := make([]float64, len(b)+1)
	cur := make([]float64, len(b)+1)
	for j := 1; j <= len(b); j++ {
		prev[j] = prev[j-1] - companyGap(b[j-1], false)
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = prev[0] - companyGap(a[i-1], true)
		for j := 1; j <= len(b); j++ {
			cur[j] = math.Max(prev[j-1]+companyTokenValue(a[i-1], b[j-1]), math.Max(prev[j]-companyGap(a[i-1], true), cur[j-1]-companyGap(b[j-1], false)))
		}
		prev, cur = cur, prev
	}
	return math.Round((100 + 100*prev[len(b)]/float64(len(a))) / 2)
}
func companyMarked(words []string) []rune {
	n := len(words)
	for _, w := range words {
		n += len(w)
	}
	out := make([]rune, 0, n)
	for i, w := range words {
		if i > 0 {
			out = append(out, ' ')
		}
		vowel := true
		var last rune
		for _, r := range w {
			if r == last {
				continue
			}
			last = r
			if strings.ContainsRune("aeiou", r) {
				out = append(out, r)
				vowel = true
			} else {
				if vowel {
					r = unicode.ToUpper(r)
				}
				out = append(out, r)
				vowel = false
			}
		}
	}
	return out
}
func companySAPS(a, b []string) float64 {
	x, y := companyMarked(a), companyMarked(b)
	if len(x) == 0 {
		if len(y) == 0 {
			return 100
		}
		return 0
	}
	var ubuf [96]bool
	var dbuf [2 * 49]int
	upper := ubuf[:0]
	if len(x)+len(y) > len(ubuf) {
		upper = make([]bool, 0, len(x)+len(y))
	}
	for _, r := range x {
		upper = append(upper, unicode.IsUpper(r))
	}
	for _, r := range y {
		upper = append(upper, unicode.IsUpper(r))
	}
	ux, uy := upper[:len(x)], upper[len(x):]
	gap := func(u bool) int {
		if u {
			return -3
		}
		return -1
	}
	rows := dbuf[:]
	if 2*(len(y)+1) > len(rows) {
		rows = make([]int, 2*(len(y)+1))
	}
	prev, cur := rows[:len(y)+1], rows[len(y)+1:2*(len(y)+1)]
	prev[0] = 0
	for j := 1; j <= len(y); j++ {
		prev[j] = prev[j-1] + gap(uy[j-1])
	}
	self := 0
	for i := 1; i <= len(x); i++ {
		u, xr := ux[i-1], x[i-1]
		gx := gap(u)
		if u {
			self += 6
		} else {
			self++
		}
		cur[0] = prev[0] + gx
		for j := 1; j <= len(y); j++ {
			v := uy[j-1]
			score := -1
			switch {
			case u != v:
				score = -4
			case u && xr == y[j-1]:
				score = 6
			case u:
				score = -2
			case xr == y[j-1]:
				score = 1
			}
			cur[j] = max(prev[j-1]+score, prev[j]+gx, cur[j-1]+gap(v))
		}
		prev, cur = cur, prev
	}
	return math.Round((100 + 100*float64(prev[len(y)])/float64(self)) / 2)
}

// companyFitScore returns the unfloored confidence. The display floor does not
// participate in the match decision.
func companyFitScore(a, b string) float64 {
	if strings.EqualFold(strings.TrimSpace(a), strings.TrimSpace(b)) {
		return 100
	}
	if score, ok := companyDigitScore(a, b); ok {
		return score
	}
	wa, wb := companyScoreWords(a, false), companyScoreWords(b, false)
	if len(wa) == 0 || len(wb) == 0 {
		return 0
	}
	if score, ok := companyQualifierScore(a, b); ok {
		return score
	}
	if strings.ContainsAny(a, "-_") != strings.ContainsAny(b, "-_") {
		clean := func(s string) string {
			return strings.ReplaceAll(strings.ReplaceAll(strings.ToLower(s), "-", ""), "_", "")
		}
		if clean(a) == clean(b) {
			return 100
		}
		if flat := strings.Join(wa, ""); flat == strings.Join(wb, "") {
			for _, r := range flat {
				if unicode.IsDigit(r) {
					return 97
				}
			}
			return 98
		}
	}
	token := companyTokenScore(wa, wb)
	best := math.Max(token, companySAPS(wa, wb))
	if containsCompanyWord(wa, "pty") != containsCompanyWord(wb, "pty") || containsCompanyWord(wa, "pvt") != containsCompanyWord(wb, "pvt") {
		best = token
	}
	if strings.Contains(a, "#") != strings.Contains(b, "#") {
		best -= 3
	}
	return best
}
func containsCompanyWord(w []string, s string) bool {
	for _, v := range w {
		if v == s {
			return true
		}
	}
	return false
}
func companyDigitScore(a, b string) (float64, bool) {
	// Split a final digit run from its preceding name and track its separator.
	split := func(s string) (string, string, byte) {
		s = strings.ToLower(strings.TrimSpace(s))
		i := len(s)
		for i > 0 && s[i-1] >= '0' && s[i-1] <= '9' {
			i--
		}
		if i == len(s) || i == 0 {
			return "", "", 0
		}
		sep := byte(0)
		if strings.ContainsRune(" -_", rune(s[i-1])) {
			sep = s[i-1]
			i--
		}
		return s[:i], strings.TrimLeft(s[i:], " -_"), sep
	}
	x, dx, sx := split(a)
	y, dy, sy := split(b)
	if score, ok := companyJoinLetterScore(a, b, x, y, dx, dy, sx, sy); ok {
		return score, true
	}
	if dx == "" || dy == "" || x != y {
		return 0, false
	}
	if (sx == '-' || sx == '_') != (sy == '-' || sy == '_') && (sx == ' ' || sy == ' ') {
		if dx == dy {
			return 97, true
		}
		if sx == ' ' {
			return 91, true
		}
		return 94, true
	}
	if (sx == ' ' && sy == 0) || (sx == 0 && sy == ' ') {
		if dx == dy && len(strings.Fields(x)) == 1 {
			if sx == ' ' {
				return 94, true
			}
			return 97, true
		}
		return 0, false
	}
	if sx == 0 && sy == 0 && len(strings.Fields(x)) > 1 && !strings.HasSuffix(x, "#") {
		// NATIVE company_name_fit/org.tsv:4: "Test Account1" and
		// "Test Account2" match at 100 in both directions. R2_glu: so do
		// "Test Account12" and "Test Account13", and "Test Account123" and
		// "Test Account124".
		if companyOneDigitApart(dx, dy) {
			return 100, true
		}
		w := strings.Fields(x)
		return companyGluedScore(a, b, dx, dy, companyDesignators[w[len(w)-1]]), true
	}
	if sx != 0 || sy != 0 || len(strings.Fields(x)) != 1 || len([]rune(x)) < 6 {
		return 0, false
	}
	if dx == dy {
		return 100, true
	}
	if len(dx) == len(dy) {
		if len([]rune(x)) >= 10 {
			return 97, true
		}
		return 95, true
	}
	if strings.HasPrefix(dx, dy) || strings.HasPrefix(dy, dx) {
		if len(dx) < len(dy) {
			return 98, true
		}
		return 96, true
	}
	if len(dx) < len(dy) {
		return 93, true
	}
	return 91, true
}

// companyOneDigitApart reports two digit runs of one length that differ in
// one position.
func companyOneDigitApart(x, y string) bool {
	if len(x) != len(y) {
		return false
	}
	n := 0
	for i := 0; i < len(x); i++ {
		if x[i] != y[i] {
			n++
		}
	}
	return n == 1
}

// companyGluedScore scores two digit runs glued to one multiword base that
// are not one position apart.
// NATIVE company_name_fit/org.tsv R2_glu (301 of 302 directed pairs), R3_glu
// and R4_glu: runs where one starts the other or adds one digit to it score
// 100; other runs score the syllable alignment of the whole names ("Test
// Account11" and "Test Account22" 97, "Test Account1" and "Test Account20"
// 95, "Test Account456" and "Test Account1" 91). After a legal designator
// only a run that starts the other scores 100: "Test Account Inc1" and "Test
// Account Inc12" 100, "Test Account Inc2" and "Test Account Inc12" 99 and 98.
func companyGluedScore(a, b, dx, dy string, designator bool) float64 {
	if strings.HasPrefix(dx, dy) || strings.HasPrefix(dy, dx) || !designator && companyDigitsNear(dx, dy) {
		return 100
	}
	return companySAPS(companyScoreWords(a, false), companyScoreWords(b, false))
}

// companyDigitsNear reports two digit runs where one starts the other or one
// is the other with a single digit added. NATIVE company_name_fit/org.tsv
// R2_glu: "Test Account1" and "Test Account123" match at 100 both ways, as do
// "Test Account1" and "Test Account21", and "Test Account13" and "Test
// Account123"; "Test Account3" and "Test Account123" score 97.
func companyDigitsNear(x, y string) bool {
	if strings.HasPrefix(x, y) || strings.HasPrefix(y, x) {
		return true
	}
	if len(x) > len(y) {
		x, y = y, x
	}
	if len(y) != len(x)+1 {
		return false
	}
	i := 0
	for i < len(x) && x[i] == y[i] {
		i++
	}
	return x[i:] == y[i+1:]
}

// companyVowel reports a lower-case vowel or "y".
func companyVowel(c byte) bool {
	return c == 'a' || c == 'e' || c == 'i' || c == 'o' || c == 'u' || c == 'y'
}

// companyJoinLetterScore is the hyphen or underscore digit join against a
// single trailing consonant on the same base. NATIVE
// company_name_fit/org.tsv:12 (B1_co1_b): "Test Account_2" and "Test Account B"
// match at 91; "Test Account A" matches the join at 100. R2_let: the other
// consonants score 91 in both directions; a vowel or "Y" scores 94 or 97 and
// keeps the earlier score.
func companyJoinLetterScore(a, b, x, y, dx, dy string, sx, sy byte) (float64, bool) {
	letter := func(s string) (string, bool) {
		s = strings.ToLower(strings.TrimSpace(s))
		i := strings.LastIndexByte(s, ' ')
		if i < 0 || len(s[i+1:]) != 1 || s[i+1] < 'a' || s[i+1] > 'z' || companyVowel(s[i+1]) {
			return "", false
		}
		return s[:i], true
	}
	if dx != "" && (sx == '-' || sx == '_') && dy == "" {
		if base, ok := letter(b); ok && base == x {
			return 91, true
		}
	}
	if dy != "" && (sy == '-' || sy == '_') && dx == "" {
		if base, ok := letter(a); ok && base == y {
			return 91, true
		}
	}
	return 0, false
}

type companyQualifier struct{ base, suffix, form string }

func splitCompanyQualifier(s string) companyQualifier {
	s = strings.TrimSpace(strings.ToLower(s))
	if i := strings.LastIndex(s, "/"); i >= 0 {
		return companyQualifier{s[:i], s[i+1:], "slash"}
	}
	if strings.HasSuffix(s, ")") {
		if i := strings.LastIndex(s, "("); i >= 0 {
			return companyQualifier{s[:i], s[i+1 : len(s)-1], "paren"}
		}
	}
	if i := strings.LastIndex(s, " - "); i >= 0 {
		return companyQualifier{s[:i], s[i+3:], "dash"}
	}
	return companyQualifier{s, "", "plain"}
}
func companyQualifierScore(a, b string) (float64, bool) {
	x, y := splitCompanyQualifier(a), splitCompanyQualifier(b)
	if x.form == "plain" && y.form == "plain" {
		return 0, false
	}
	words := func(s string) string { return strings.Join(companyScoreWords(s, false), " ") }
	if x.form == "plain" || y.form == "plain" {
		p, q := x, y
		if p.form != "plain" {
			p, q = q, p
		}
		if words(p.base) == words(q.base) {
			if q.form == "slash" {
				return 100, true
			}
			if x.form == "plain" {
				return 98, true
			}
			return 97, true
		}
		// Treat the final plain word as a suffix.
		w := strings.Fields(p.base)
		if len(w) < 2 {
			return 0, false
		}
		p.base = strings.Join(w[:len(w)-1], " ")
		p.suffix = w[len(w)-1]
		if x.form == "plain" {
			x = p
		} else {
			y = p
		}
	}
	if words(x.base) != words(y.base) || x.suffix == "" || y.suffix == "" {
		return 0, false
	}
	n := len([]rune(words(x.suffix)))
	if x.form == y.form {
		return 100, true
	}
	score := 96.0
	switch x.form + ":" + y.form {
	case "plain:slash", "slash:plain":
		score = 98
		if n >= 8 {
			score = 99
		}
	case "slash:paren", "slash:dash":
		score = 97
		if n >= 8 {
			score = 98
		}
	case "paren:slash", "dash:slash":
		score = 95

		if n >= 8 {
			score = 97
		}
	case "plain:paren", "plain:dash":
		score = 98
	case "paren:plain", "dash:plain":
		score = 97
	}
	if words(x.suffix) != words(y.suffix) {
		switch x.form {
		case "plain":
			score -= 4
		case "slash":
			score -= 7
		default:
			score -= 6
		}
	}
	return score, true
}

// companyFitThreshold is the documented Company Name threshold of 70 read as
// a confidence, (100 + 70) / 2; that reading is fitted, not documented.
// NATIVE B3_cw1: on the join route the one pair at 85 matches and the three
// at 83 do not. The score does not separate pairs outside the fitted routes
// (company_name_fit has rejected pairs at 85 and matched pairs at 77 there).
const companyFitThreshold = 85

// companyFitCovered keeps a fitted match to the pair shapes the panels
// cover; any other pair keeps the word scorer's decision.
//
// Shape route: the captured numbered and designator pairs differ in their
// numbers, joins and designators, not in their words. NATIVE B3_cw1 and
// B1_co3: names that share the six-character key but differ past it
// ("Qelthorin Freights" and "Qelthorin Freighter", "Testing Account" and
// "Tester Account") do not match, and neither do "Qelthorin Freight Systems"
// and "Qelthorin Freight Services Group". So a number does not make a match
// of two names the word scorer keeps apart without it, and the first two key
// words of a designator pair must agree in full.
//
// Qualifier route: the captured cross-form pairs share one suffix. NATIVE
// B3_cw1: a " - " suffix rejects a different plain last word in both
// directions but one, so a " - " side with a different suffix is not scored.
func companyFitCovered(route int, a, b string) bool {
	switch route {
	case companyRouteShape:
		if (companyMarks(a)|companyMarks(b))&companyMarkDigit != 0 {
			x, y := companyUnnumbered(a), companyUnnumbered(b)
			if x == "" || y == "" || foldedEqual(x, y) {
				return true
			}
			// NATIVE company_name_fit/org.tsv D1_nd: with a differing
			// word, both names carry a number and the numbers are equal or
			// one starts the other.
			if companyNumberGate(a, b) {
				return false
			}
			ok, _ := legacyFuzzyCompany(x, y, false)
			return ok
		}
		wa, wb := companyScoreWords(a, true), companyScoreWords(b, true)
		return len(wa) > 0 && len(wb) > 0 && wa[0] == wb[0] && (len(wa) < 2 || len(wb) < 2 || wa[1] == wb[1])
	case companyRouteJoin:
		// NATIVE company_name_fit/org.tsv D1_jn: an underscore join does not
		// match its spaced name ("Zorvex_Marine Supply"), and a hyphen join
		// matches only when the aligned words are alike ("Vexlett-Packard" and
		// "Vexlet Packard" match; "Quen-State Paving" and "Quen State
		// Painting" do not).
		if strings.Contains(a, "_") || strings.Contains(b, "_") {
			return false
		}
		wa := strings.Fields(strings.ToLower(strings.ReplaceAll(a, "-", " ")))
		wb := strings.Fields(strings.ToLower(strings.ReplaceAll(b, "-", " ")))
		for i := 0; i < len(wa) && i < len(wb); i++ {
			if wa[i] != wb[i] && !strings.HasPrefix(wa[i], wb[i]) && !strings.HasPrefix(wb[i], wa[i]) && jaroWinkler(wa[i], wb[i]) < .9 {
				return false
			}
		}
	case companyRouteQualifier:
		x, y := splitCompanyQualifier(a), splitCompanyQualifier(b)
		sx, sy := companyScoreWords(x.suffix, false), companyScoreWords(y.suffix, false)
		bx, by := companyScoreWords(x.base, false), companyScoreWords(y.base, false)
		if x.form == "dash" || y.form == "dash" || strings.Contains(a, " / ") || strings.Contains(b, " / ") {
			// NATIVE D1_qf, R1_dir, R2_vow, R2_par: two different one-word
			// suffixes match across a dash or a spaced slash on the same
			// base as companySuffixFinds describes.
			// A dash form finds an unspaced slash form only at the same suffix.
			// NATIVE D1_qf: "Zorvex Freight - West" finds "Zorvex Freights/West".
			if x.form == "dash" && y.form == "slash" && !strings.Contains(b, " / ") {
				return companyAlikeWords(bx, by) && strings.Join(sx, " ") == strings.Join(sy, " ")
			}
			return companyDashSuffixCovered(x, y, bx, by, sx, sy)
		}
		// NATIVE company_name_fit/org.tsv D1_qf: a slash and a parenthesis
		// form match when the suffixes have the same number of words and the
		// bases are alike word by word ("Freight" and "Freights"); a longer
		// suffix ("West Coast") or a longer base ("Freight Services") does
		// not match.
		// NATIVE R1_qf2: a slash after a single letter ("c/o West") is not
		// a qualifier; "Orbel Trade (West)" finds "Orbel Trade c/o West".
		if companySlashLetter(x) || companySlashLetter(y) {
			return true
		}
		if len(sx) != len(sy) || !companyAlikeWords(bx, by) {
			return false
		}
	}
	return true
}

// companyDashSuffixCovered is the dash and spaced slash part of the
// qualifier route: one base, suffixes of one word count, and equal suffixes
// or a suffix that companySuffixFinds accepts.
func companyDashSuffixCovered(x, y companyQualifier, bx, by, sx, sy []string) bool {
	if strings.Join(bx, " ") != strings.Join(by, " ") || len(sx) != len(sy) || len(sx) == 0 {
		return false
	}
	if strings.Join(sx, " ") == strings.Join(sy, " ") {
		return true
	}
	if len(sx) != 1 {
		return false
	}
	return companySuffixFinds(sx[0], sy[0], x.form == "paren" || y.form == "paren", x.form == "slash" || y.form == "slash")
}

// companySuffixFinds reports whether a candidate suffix finds a different
// stored suffix across two qualifier forms.
//
// NATIVE company_name_fit/org.tsv R1_dir, R2_vow and R2_par, dash against
// spaced slash: suffixes that start with consonants match in both directions
// when they are alike ("West", "Best" and "Nest"; "Mast" and "Last"; not
// "West" and "Mast" or "North"). A suffix that starts with a vowel finds an
// alike consonant suffix ("East" finds "West" and "Mast") and is not found by
// it. Two vowel suffixes match when only the first letter differs ("East",
// "Oast", "Uast"); "East" does not find "Ease".
//
// Against a parenthesis form only suffixes with the same consonants match
// ("West" and "Wast"): "Orbel Trade - East" does not find "Orbel Trade (West)".
// A vowel suffix between a spaced slash and a parenthesis keeps the rule above
// and with it the earlier decision: R2_par matches "Zorvex Freight (East)" and
// "Zorvex Freight / West", R1_dir does not match the same pair on "Orbel Trade".
// R4 to R7: a vowel candidate across a dash and a parenthesis form matches on
// some bases only; it stays unmatched.
func companySuffixFinds(candidate, stored string, paren, spaced bool) bool {
	if candidate == "" || stored == "" || !isASCII(candidate) || !isASCII(stored) {
		return false
	}
	vc, vs := companyVowel(candidate[0]), companyVowel(stored[0])
	switch {
	case paren && !(spaced && vc):
		return !vc && !vs && companyConsonants(candidate) == companyConsonants(stored)
	case vs && !vc:
		return false
	case vs:
		return candidate[1:] == stored[1:]
	}
	return jaroWinkler(candidate, stored) >= .8
}

// companyConsonants is a word without its vowels.
func companyConsonants(s string) string {
	return strings.Map(func(r rune) rune {
		if r < 0x80 && companyVowel(byte(r)) {
			return -1
		}
		return r
	}, s)
}

// companyAlikeWords reports equal word counts where each word starts the
// word across from it or is started by it ("freight" and "freights").
func companyAlikeWords(x, y []string) bool {
	if len(x) != len(y) {
		return false
	}
	for i := range x {
		if !strings.HasPrefix(x[i], y[i]) && !strings.HasPrefix(y[i], x[i]) {
			return false
		}
	}
	return true
}

// companySlashLetter reports a slash form whose slash follows a single letter.
func companySlashLetter(q companyQualifier) bool {
	n := len(q.base)
	return q.form == "slash" && n > 0 && q.base[n-1] != ' ' && (n == 1 || q.base[n-2] == ' ')
}

// companyNumberGate reports a numbered pair with a differing word whose
// numbers do not relate. NATIVE company_name_fit/org.tsv D1_nd covers names
// of up to three words; R1_num and R1_dig keep the fitted decision for longer
// names ("Qorvan Depot 4 North" and "Qorvan Depot 3 North East" match), and a
// "#", a "No" before a number and a number with a letter ("3A") are part of
// the number, not a differing word. R2_let: neither is a Roman numeral
// ("Test Account II" and "Test Account-1" match).
func companyNumberGate(a, b string) bool {
	x, markedA := companyGateWords(a)
	y, markedB := companyGateWords(b)
	if markedA || markedB || len(x) > 3 || len(y) > 3 || slices.Equal(x, y) {
		return false
	}
	gluedA, gluedB := companyGlued(a), companyGlued(b)
	last := len(y) - 1
	switch {
	case gluedA && !gluedB && len(x) == last && slices.Equal(x, y[:last]) && companyVowel(y[last][0]) &&
		strings.TrimLeft(companyFirstNumber(a), "0") == strings.TrimLeft(companyFirstNumber(b), "0"):
		// NATIVE D1_nd and R2_vow: "Zorvex Region1" finds "Zorvex Region 1
		// East" and "Zorvex Region 1 Oast", not "Zorvex Region 1 West".
		return false
	case !gluedA && !gluedB && len(x) == 3 && len(y) == 3 && slices.Equal(x[:2], y[:2]) &&
		companyVowel(x[2][0]) && !companyVowel(y[2][0]) && companySuffixFinds(x[2], y[2], false, false):
		// NATIVE D1_nd and R2_vow: "Zorvex Region 2 East" finds "Zorvex
		// Region 1 West" and "Zorvex Region 1 Oast" finds "Zorvex Region 2
		// Mast"; neither is found in return.
		return false
	}
	return !companyNumbersRelated(a, b) || gluedA || gluedB
}

// companyGateWords is the words of a name without its numbers, number
// markers and single characters. marked reports a "No" before a number; such
// a pair keeps the fitted decision (NATIVE R1_num: "Qorvan Depot No 3" and
// "Qorvan Depot North" match).
func companyGateWords(s string) (out []string, marked bool) {
	fields := strings.Fields(strings.ToLower(s))
	for i, w := range fields {
		if (w == "no" || w == "no.") && i+1 < len(fields) && companyFirstNumber(fields[i+1][:1]) != "" {
			marked = true
			continue
		}
		w = companyUnnumbered(w)
		w = strings.Trim(w, "#")
		if utf8.RuneCountInString(w) > 1 && companyFirstNumber(w) == "" && strings.Trim(w, "ivx") != "" {
			out = append(out, w)
		}
	}
	return out, marked
}

// companyNumbersRelated reports that both names hold a number and that, with
// leading zeros dropped, the first numbers are equal or one starts the other.
func companyNumbersRelated(a, b string) bool {
	x, y := companyFirstNumber(a), companyFirstNumber(b)
	if x == "" || y == "" {
		return false
	}
	x, y = strings.TrimLeft(x, "0"), strings.TrimLeft(y, "0")
	return strings.HasPrefix(x, y) || strings.HasPrefix(y, x)
}

// companyGlued reports a digit run attached to a letter ("Region1"). NATIVE
// company_name_fit/org.tsv D1_nd: "Zorvex Region1" does not match "Zorvex Region
// 1 Sales" or "Zorvex Region 1 West".
func companyGlued(s string) bool {
	for i := 1; i < len(s); i++ {
		if s[i] >= '0' && s[i] <= '9' && (s[i-1]|0x20 >= 'a' && s[i-1]|0x20 <= 'z') {
			return true
		}
	}
	return false
}

// companyFirstNumber is the first run of ASCII digits in s.
func companyFirstNumber(s string) string {
	start := strings.IndexFunc(s, func(r rune) bool { return r >= '0' && r <= '9' })
	if start < 0 {
		return ""
	}
	end := start
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	return s[start:end]
}

// companyUnnumbered drops the numbers from a name: a "#", a number word, a
// trailing digit run with its hyphen or underscore ("Account-1", "Account1")
// and a leading one with its hyphen or underscore ("7-Zorval").
func companyUnnumbered(s string) string {
	var out []string
	for _, w := range strings.Fields(s) {
		w = strings.TrimLeft(w, "#")
		w = strings.TrimRight(strings.TrimRight(w, "0123456789"), "-_")
		if t := strings.TrimLeft(w, "0123456789"); t != w && (t == "" || t[0] == '-' || t[0] == '_') {
			w = strings.TrimLeft(t, "-_")
		}
		if w != "" {
			out = append(out, w)
		}
	}
	return strings.Join(out, " ")
}

// Fitted routes: the pair shapes the captured panels cover. A shape route has
// a digit or a trailing Partner, SARL or BV designator; a join route has a
// hyphen or underscore join on one side only; a qualifier route has two
// different trailing qualifiers.
const (
	companyRouteNone = iota
	companyRouteShape
	companyRouteJoin
	companyRouteQualifier
)

func companyFitRoute(a, b string) int {
	return companyFitRouteMarked(a, b, companyMarks(a), companyMarks(b))
}

func companyFitRouteMarked(a, b string, ma, mb int) int {
	switch {
	case companyFitShapeMarked(a, b, ma, mb):
		return companyRouteShape
	case (ma|mb)&(companyMarkJoin|companyMarkQualifier) == 0:
		return companyRouteNone
	case companyJoinShape(a, b, ma, mb):
		return companyRouteJoin
	case companyQualifierPair(a, b):
		return companyRouteQualifier
	}
	return companyRouteNone
}

// companyFitShape is the numeric and designator part of the route.
func companyFitShape(a, b string) bool {
	return companyFitShapeMarked(a, b, companyMarks(a), companyMarks(b))
}

func companyFitShapeMarked(a, b string, ma, mb int) bool {
	return (ma|mb)&companyMarkDigit != 0 || companyDesignatorShape(a) || companyDesignatorShape(b)
}

const (
	companyMarkDigit = 1 << iota
	companyMarkJoin
	companyMarkQualifier
)

// companySpacedHyphen reports a hyphen after a space, as in " - ".
func companySpacedHyphen(s string) bool {
	for i := strings.IndexByte(s, '-'); i >= 0; {
		if i > 0 && s[i-1] == ' ' {
			return true
		}
		next := strings.IndexByte(s[i+1:], '-')
		if next < 0 {
			return false
		}
		i += next + 1
	}
	return false
}

func companyMarks(s string) int {
	m := 0
	for i := 0; i < len(s); i++ {
		switch c := s[i]; {
		case c >= '0' && c <= '9':
			m |= companyMarkDigit
		case c == '-' || c == '_':
			m |= companyMarkJoin
		case c == '/' || c == ')':
			m |= companyMarkQualifier
		case c >= 0x80:
			r, _ := utf8.DecodeRuneInString(s[i:])
			if unicode.IsDigit(r) {
				m |= companyMarkDigit
			}
		}
	}
	return m
}

// companyDesignatorShape reports a trailing Partner, SARL or BV after at
// least one other word.
func companyDesignatorShape(s string) bool {
	s = strings.TrimSpace(s)
	i := strings.LastIndexFunc(s, unicode.IsSpace)
	if i < 0 || strings.TrimSpace(strings.ReplaceAll(s[:i], ".", "")) == "" {
		return false
	}
	_, width := utf8.DecodeRuneInString(s[i:])
	last := strings.ReplaceAll(s[i+width:], ".", "")
	return strings.EqualFold(last, "partner") || strings.EqualFold(last, "sarl") || strings.EqualFold(last, "bv")
}

// companyJoinShape is a hyphen or underscore join on one side only, with no
// " - " suffix on either side. NATIVE duplicate_fuzzy_matching/org.tsv:108:
// a joined side with more words than the other ("Dunder-Mifflin Paper" and
// "Dunder Mifflin") does not match; companyJoinWords keeps that shape on the
// word scorer.
func companyJoinShape(a, b string, ma, mb int) bool {
	return (ma&companyMarkJoin != 0) != (mb&companyMarkJoin != 0) && !strings.Contains(a, " - ") && !strings.Contains(b, " - ")
}

// companyJoinWords reports whether the joined side has no more key words
// than the other side.
func companyJoinWords(a string, wordsA, wordsB int) bool {
	if strings.ContainsAny(a, "-_") {
		return wordsA <= wordsB
	}
	return wordsB <= wordsA
}

// companyQualifierPair is two different trailing qualifier forms (slash,
// parenthesis, " - ") on the same base.
func companyQualifierPair(a, b string) bool {
	x, y := companyQualifierForm(a), companyQualifierForm(b)
	return x != "plain" && y != "plain" && x != y
}

// companyQualifierForm is splitCompanyQualifier's form without the split.
func companyQualifierForm(s string) string {
	s = strings.TrimSpace(s)
	switch {
	case strings.Contains(s, "/"):
		return "slash"
	case strings.HasSuffix(s, ")") && strings.Contains(s, "("):
		return "paren"
	case strings.Contains(s, " - "):
		return "dash"
	}
	return "plain"
}

// companyJoinConfidence reports a join-route match. NATIVE B3_cw1 and
// duplicate_fuzzy_matching/org.tsv:75: when the two names with the join
// spelled as a space match at 100 the pair reports 95; when they match lower,
// that confidence; otherwise the fitted score.
func companyJoinConfidence(a, b string, fitted float64) float64 {
	space := func(s string) string {
		return strings.Map(func(r rune) rune {
			if r == '-' || r == '_' {
				return ' '
			}
			return r
		}, s)
	}
	if ok, confidence := legacyFuzzyCompany(space(a), space(b), false); ok {
		return math.Min(95, confidence)
	}
	return math.Max(91, fitted)
}

// companyKeyBuffer bounds the names companyFitKeys handles without
// allocating; longer or non-ASCII names use companyScoreWords.
const companyKeyBuffer = 128

// companyFitKeys reports whether the two names share the fitted match key, and
// the number of key words in each.
func companyFitKeys(a, b string) (same bool, wordsA, wordsB int) {
	var bufA, bufB [companyKeyBuffer]byte
	ka, na, okA := companyFitKeyASCII(a, bufA[:0])
	kb, nb, okB := companyFitKeyASCII(b, bufB[:0])
	if okA && okB {
		return string(ka) == string(kb), na, nb
	}
	wa, wb := companyScoreWords(a, true), companyScoreWords(b, true)
	return companyFitKeyOf(wa) == companyFitKeyOf(wb), len(wa), len(wb)
}

// companyFitKeyASCII is companyFitKey and the key word count for an ASCII
// name, written into dst. ok is false when the name needs the general path.
func companyFitKeyASCII(s string, dst []byte) (key []byte, words int, ok bool) {
	if len(s) > companyKeyBuffer/2 {
		return nil, 0, false
	}
	var buf [companyKeyBuffer]byte
	var spans [16][2]int
	n, w, start := 0, 0, -1
	end := func() bool {
		if start >= 0 && n > start {
			if w == len(spans) {
				return false
			}
			spans[w] = [2]int{start, n}
			w++
		}
		start = -1
		return true
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 0x80:
			return nil, 0, false
		case c >= 'A' && c <= 'Z':
			c += 'a' - 'A'
			fallthrough
		case c >= 'a' && c <= 'z' || c >= '0' && c <= '9':
			if start < 0 {
				start = n
			}
			buf[n] = c
			n++
		case c == '.':
		case c == '&':
			if n+3 > len(buf) || !end() {
				return nil, 0, false
			}
			start = n
			n += copy(buf[n:], "and")
			if !end() {
				return nil, 0, false
			}
		default:
			if !end() {
				return nil, 0, false
			}
		}
	}
	if !end() {
		return nil, 0, false
	}
	word := func(i int) string { return string(buf[spans[i][0]:spans[i][1]]) }
	is := func(i int, v string) bool { return string(buf[spans[i][0]:spans[i][1]]) == v }
	for w > 1 {
		switch {
		case w > 2 && (is(w-2, "pvt") && is(w-1, "ltd") || is(w-2, "private") && is(w-1, "limited") || is(w-2, "s") && is(w-1, "a") || is(w-2, "b") && is(w-1, "v") || is(w-2, "incorporated") && is(w-1, "company")):
			w -= 2
			continue
		case w > 3 && is(w-3, "l") && is(w-2, "l") && is(w-1, "c"):
			w -= 3
			continue
		case companyDesignators[word(w-1)] || is(w-1, "partner") || is(w-1, "sarl") || is(w-1, "bv"):
			w--
			continue
		}
		break
	}
	first := 0
	if w > 1 && is(0, "the") {
		first = 1
	}
	kept := 0
	key = dst
	for i := first; i < w; i++ {
		if is(i, "and") || is(i, "of") || is(i, "the") || is(i, "a") {
			continue
		}
		if kept < 2 {
			key = append(key, buf[spans[i][0]:min(spans[i][1], spans[i][0]+6)]...)
		}
		kept++
	}
	if kept == 0 {
		for i := first; i < min(w, first+2); i++ {
			key = append(key, buf[spans[i][0]:min(spans[i][1], spans[i][0]+6)]...)
		}
		kept = w - first
	}
	return key, kept, true
}

// companyStemFirstWord returns the candidate with its first word replaced by
// the stored name's first word when the two first words share the six-letter
// match key and one is the other with letters added after it, or both have
// one length and differ in one letter after it. Both names must have at
// least two words of ASCII letters only.
//
// NATIVE company_name_fit/org.tsv R1_dbl and R3_key (custom CompanyName rule,
// API 62.0 and 67.0, both routes): "Vexlet Packard" and each of "Vexlets",
// "Vexleta", "Vexletab", "Vexletcd", "Vexleter", "Vexletronics" and
// "Vexlettt Packard" match at 100 in both directions, as do "Vexleta" and
// "Vexletab", "Vexlets" and "Vexleta", "Vexlett" and "Vexlets", "Qelthorin
// Freight" and "Qelthorins Freight", and "Zorvexs Marine Supply" and
// "Zorvexa Marine Supply". The rest of the name keeps the word scorer's
// rules: "Vexlet Packardd" finds "Vexlett Packard" at 100 and "Vexlet
// Packard Group" finds "Vexleta Packard" at 91. Not matched: "Vexlets" and
// "Vexleter", "Vexletab" and "Vexletcd" (two letters), "Vexle" and
// "Vexles" (five letters), and the one-word names "Vexlets" and "Vexlet".
func companyStemFirstWord(a, b string) (string, bool) {
	// first is the leading run of letters after any spaces.
	first := func(s string) (string, int) {
		at := 0
		for at < len(s) && s[at] == ' ' {
			at++
		}
		end := at
		for end < len(s) && (s[end]|0x20 >= 'a' && s[end]|0x20 <= 'z') {
			end++
		}
		return s[at:end], at
	}
	// shaped reports two or more words of ASCII letters and spaces only.
	shaped := func(s string) bool {
		words := 0
		for i := 0; i < len(s); i++ {
			switch c := s[i]; {
			case c == ' ':
			case c|0x20 >= 'a' && c|0x20 <= 'z':
				if i == 0 || s[i-1] == ' ' {
					words++
				}
			default:
				return false
			}
		}
		return words >= 2
	}
	x, at := first(a)
	y, _ := first(b)
	if len(x) < 6 || len(y) < 6 {
		return "", false
	}
	lower := func(c byte) byte {
		if c >= 'A' && c <= 'Z' {
			return c + 'a' - 'A'
		}
		return c
	}
	common := 0
	for common < len(x) && common < len(y) && lower(x[common]) == lower(y[common]) {
		common++
	}
	if common < 6 || common == len(x) && common == len(y) {
		return "", false
	}
	if common < len(x) && common < len(y) {
		// Neither starts the other: one length, one differing letter.
		if len(x) != len(y) {
			return "", false
		}
		for i := common + 1; i < len(x); i++ {
			if lower(x[i]) != lower(y[i]) {
				return "", false
			}
		}
	}
	if !shaped(a) || !shaped(b) || companyAlignedWords[strings.ToLower(x)] || companyAlignedWords[strings.ToLower(y)] {
		return "", false
	}
	return a[:at] + y + a[at+len(x):], true
}
