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
		letters := utf8.RuneCountInString(bx[0]) + utf8.RuneCountInString(bx[1])
		if letters >= 13 {
			return true, 90
		}
		return false, 0
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
