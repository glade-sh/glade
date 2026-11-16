package dml

import (
	"strings"
	"unicode/utf8"
)

// Company Name ceilings: pairs the fitted scorer matched that the native rows
// reject (company_name_fit R4_misc, R4_sp, R4_glu, R7_pl and R7_sp). Each term
// below removes one group and none adds a false negative on the panels.
//
//   - companyGluedThreshold: two names that end in a digit run glued to a
//     letter ("Test Account12345") need a decision of 90, or 85 when every base
//     word on both sides has three letters or fewer ("Aeo Uia1235"). Two names
//     with a balanced two-word base ("Orbel Trade12345") add one point.
//   - companyCeiling: a qualifier or direction pair that the scorer matched.
//     A spaced slash against a parenthesis form on one two-word base with
//     different suffixes is no match on a base under 12 letters; on 12 or 13
//     letters the spaced slash candidate reports 90 (fuzzy_residual_panels
//     r2co_p1: "Zorvex Fright / East" finds "Zorvex Fright (West)" at 90, and
//     "Zorvex Freights / East" its form at 91). The parenthesis candidate and
//     two suffixes that differ in one vowel keep the scorer's result
//     (company_name_fit R2_par: "(East)" finds "/ Mast" at 91, "/ East" finds
//     "(Oast)" at 93).

// companyVowelWords are the qualifiers the fitted vowel and consonant
// suffix families cover; a same-qualifier pair with another word of four
// letters or more (dash against parenthesis) is not a match.
var companyQualifierWords = map[string]bool{
	"east": true, "west": true, "north": true, "south": true,
	"best": true, "nest": true, "wast": true, "mast": true,
	"last": true, "oast": true, "uast": true,
}

var companyDirections = map[string]bool{"east": true, "west": true, "north": true, "south": true}

// companyGluedBase is the base words of a name that ends in a digit run glued
// to a letter, and whether it does.
func companyGluedBase(s string) ([]string, bool) {
	end := len(s)
	for end > 0 && s[end-1] >= '0' && s[end-1] <= '9' {
		end--
	}
	if end == len(s) || end == 0 {
		return nil, false
	}
	if c := s[end-1] | 0x20; c < 'a' || c > 'z' {
		return nil, false
	}
	return strings.Fields(strings.ToLower(s[:end])), true
}

// companyGluedThreshold is the match threshold and the decision bonus of the
// glued-digit shape route for a pair, or the plain threshold when either name
// does not end in a glued digit run.
func companyGluedThreshold(a, b string) (threshold, bonus float64) {
	threshold = companyFitThreshold
	if len(a) == 0 || len(b) == 0 || a[len(a)-1] < '0' || a[len(a)-1] > '9' || b[len(b)-1] < '0' || b[len(b)-1] > '9' {
		return threshold, 0
	}
	x, okX := companyGluedBase(a)
	y, okY := companyGluedBase(b)
	if !okX || !okY {
		return threshold, 0
	}
	if !companyShortBase(x) || !companyShortBase(y) {
		threshold = 90
	}
	if companyBalancedBase(x) && companyBalancedBase(y) {
		bonus = 1
	}
	return threshold, bonus
}

// companyShortBase reports base words of at most three letters each.
func companyShortBase(words []string) bool {
	for _, w := range words {
		if utf8.RuneCountInString(w) > 3 {
			return false
		}
	}
	return true
}

// companyBalancedBase reports exactly two base words of at least four letters
// whose lengths differ by at most one.
func companyBalancedBase(words []string) bool {
	if len(words) != 2 {
		return false
	}
	x, y := utf8.RuneCountInString(words[0]), utf8.RuneCountInString(words[1])
	return x >= 4 && y >= 4 && abs(x-y) <= 1
}

// companyCeiling applies the qualifier and direction terms to a pair the
// scorer matched at the displayed score. A pair that is its own text (self
// pair) is exempt.
func companyCeiling(a, b string, score float64) (bool, float64) {
	if a == b {
		return true, score
	}
	// Every term needs a qualifier form on both names or a direction word in
	// both; most matched pairs have neither and return here without
	// splitting the names.
	switch plainA, plainB := companyPlainForm(a), companyPlainForm(b); {
	case plainA != plainB:
		return true, score
	case plainA && (!companyHasDirection(a) || !companyHasDirection(b)):
		return true, score
	}
	x, y := splitCompanyQualifier(a), splitCompanyQualifier(b)
	if x.form == "plain" && y.form == "plain" {
		return companyDirectionCeiling(a, b, score)
	}
	words := func(s string) []string { return companyScoreWords(s, false) }
	bx, by := words(x.base), words(y.base)
	markedX, markedY := x.form == "dash" || x.form == "paren", y.form == "dash" || y.form == "paren"
	if markedX && markedY {
		sx, sy := strings.Join(words(x.suffix), " "), strings.Join(words(y.suffix), " ")
		if strings.Contains(sx, " ") || strings.Contains(sy, " ") {
			// NATIVE D1_qf: a qualifier of two words ("West Coast") keeps
			// the scorer's decision across both forms and base lengths.
			return true, score
		}
		short, long := bx, by
		if len(short) > len(long) {
			short, long = long, short
		}
		// A base that is a proper prefix of the other, with other qualifiers.
		if len(short) < len(long) && strings.Join(short, " ") == strings.Join(long[:len(short)], " ") && sx != sy {
			return false, 0
		}
		// The same base and the same qualifier outside the vocabulary.
		if len(bx) == len(by) && strings.Join(bx, " ") == strings.Join(by, " ") && sx == sy && !companyQualifierWords[sx] && utf8.RuneCountInString(sx) >= 4 {
			return false, 0
		}
		return true, score
	}
	if x.form == "slash" && y.form == "paren" || x.form == "paren" && y.form == "slash" {
		if !strings.Contains(a+b, " / ") || strings.Join(bx, " ") != strings.Join(by, " ") || len(bx) != 2 {
			return true, score
		}
		sx, sy := strings.Join(words(x.suffix), " "), strings.Join(words(y.suffix), " ")
		if sx == sy {
			return true, score
		}
		if companyVowelSwap(sx, sy) {
			return true, score
		}
		letters := utf8.RuneCountInString(bx[0]) + utf8.RuneCountInString(bx[1])
		if letters < 12 {
			return false, 0
		}
		if x.form == "slash" && letters <= 13 {
			return true, 90
		}
		return true, score
	}
	return true, score
}

// companyDirectionCeiling is the plain direction pair: two names without a
// marker that differ in a final direction word, on a base whose first word has
// at most two letters and that has ten letters or more, score by their
// syllable alignment and match at 91.
func companyDirectionCeiling(a, b string, score float64) (bool, float64) {
	if companyMarks(a)|companyMarks(b) != 0 {
		return true, score
	}
	wa, wb := companyScoreWords(a, false), companyScoreWords(b, false)
	if len(wa) < 2 || len(wa) != len(wb) {
		return true, score
	}
	la, lb := wa[len(wa)-1], wb[len(wb)-1]
	if la == lb || !companyDirections[la] || !companyDirections[lb] {
		return true, score
	}
	base := wa[:len(wa)-1]
	if strings.Join(base, " ") != strings.Join(wb[:len(wb)-1], " ") || utf8.RuneCountInString(base[0]) > 2 {
		return true, score
	}
	letters := 0
	for _, w := range base {
		letters += utf8.RuneCountInString(w)
	}
	if letters < 10 {
		return true, score
	}
	if item := companySAPS(wa, wb); item >= 91 {
		return true, item
	}
	return false, 0
}

// companyVowelSwap reports two qualifiers of one length that differ in one
// letter, a vowel on both sides ("East" and "Oast", "West" and "Wast").
func companyVowelSwap(a, b string) bool {
	if len(a) != len(b) {
		return false
	}
	differ := 0
	for i := 0; i < len(a); i++ {
		if a[i] == b[i] {
			continue
		}
		if !strings.ContainsRune("aeiou", rune(a[i])) || !strings.ContainsRune("aeiou", rune(b[i])) {
			return false
		}
		differ++
	}
	return differ == 1
}

// companyPlainForm reports a name splitCompanyQualifier leaves plain: no
// slash, no closing parenthesis form and no spaced dash.
func companyPlainForm(s string) bool {
	s = strings.TrimSpace(s)
	if strings.Contains(s, "/") || strings.Contains(s, " - ") {
		return false
	}
	return !strings.HasSuffix(s, ")") || !strings.Contains(s, "(")
}

// companyHasDirection reports a name that holds a direction word's letters,
// in any case: the cheap test before companyDirectionCeiling splits words.
func companyHasDirection(s string) bool {
	for i := 0; i+4 <= len(s); i++ {
		switch s[i] | 0x20 {
		case 'e', 'w':
			if strings.EqualFold(s[i+1:i+4], "ast") || strings.EqualFold(s[i+1:i+4], "est") {
				return true
			}
		case 'n', 's':
			if i+5 <= len(s) && (strings.EqualFold(s[i+1:i+5], "orth") || strings.EqualFold(s[i+1:i+5], "outh")) {
				return true
			}
		}
	}
	return false
}
