package dml

import (
	"math"
	"strings"
	"unicode"
	"unicode/utf8"
)

var companyExpansions = map[string]string{"intl": "international", "natl": "national", "univ": "university", "st": "saint", "mt": "mount", "assoc": "association", "dept": "department", "mfg": "manufacturing", "svcs": "services", "us": "unitedstates"}

// companyScoreWords normalizes words used by the directional scorer. The key
// deliberately has different punctuation handling and does not expand words.
func companyScoreWords(s string, key bool) []string {
	var b strings.Builder
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
	w := strings.Fields(b.String())
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
	out := make([]string, 0, len(w))
	for _, v := range w {
		if v != "and" && v != "of" && v != "the" && v != "a" {
			out = append(out, v)
		}
	}
	if len(out) == 0 {
		return w
	}
	return out
}
func companyFitKey(s string) string {
	w := companyScoreWords(s, true)
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
	var out []rune
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
	gap := func(r rune) int {
		if unicode.IsUpper(r) {
			return -3
		}
		return -1
	}
	prev := make([]int, len(y)+1)
	cur := make([]int, len(y)+1)
	for j := 1; j <= len(y); j++ {
		prev[j] = prev[j-1] + gap(y[j-1])
	}
	self := 0
	for i := 1; i <= len(x); i++ {
		if unicode.IsUpper(x[i-1]) {
			self += 6
		} else {
			self++
		}
		cur[0] = prev[0] + gap(x[i-1])
		for j := 1; j <= len(y); j++ {
			u, v := unicode.IsUpper(x[i-1]), unicode.IsUpper(y[j-1])
			score := -1
			switch {
			case u != v:
				score = -4
			case u && v && x[i-1] == y[j-1]:
				score = 6
			case u && v:
				score = -2
			case x[i-1] == y[j-1]:
				score = 1
			}
			cur[j] = max(prev[j-1]+score, prev[j]+gap(x[i-1]), cur[j-1]+gap(y[j-1]))
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
		flat := func(s string) string { return strings.Join(companyScoreWords(s, false), "") }
		if flat(a) == flat(b) {
			for _, r := range flat(a) {
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

// companyFitThreshold is the documented Company Name threshold of 70 as a
// confidence, (100 + 70) / 2. NATIVE B3_cw1: hyphen-joined pairs at 85 and
// above match and those at 83 and below do not.
const companyFitThreshold = 85

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
	ma, mb := companyMarks(a), companyMarks(b)
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
	last := strings.ReplaceAll(s[i+1:], ".", "")
	return strings.EqualFold(last, "partner") || strings.EqualFold(last, "sarl") || strings.EqualFold(last, "bv")
}

// companyJoinShape is a hyphen or underscore join on one side only, with no
// " - " suffix on either side. NATIVE duplicate_fuzzy_matching/org.tsv:108:
// a joined side with more words than the other ("Dunder-Mifflin Paper" and
// "Dunder Mifflin") does not match, so that shape keeps the word scorer.
func companyJoinShape(a, b string, ma, mb int) bool {
	ja, jb := ma&companyMarkJoin != 0, mb&companyMarkJoin != 0
	if ja == jb || strings.Contains(a, " - ") || strings.Contains(b, " - ") {
		return false
	}
	joined, other := a, b
	if jb {
		joined, other = b, a
	}
	return len(companyScoreWords(joined, true)) <= len(companyScoreWords(other, true))
}

// companyQualifierPair is two different trailing qualifier forms (slash,
// parenthesis, " - ") on the same base.
func companyQualifierPair(a, b string) bool {
	x, y := splitCompanyQualifier(a), splitCompanyQualifier(b)
	return x.form != "plain" && y.form != "plain" && x.form != y.form
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
