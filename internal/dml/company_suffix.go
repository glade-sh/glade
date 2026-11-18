package dml

import "strings"

// companySuffixParts splits a name into its base words, a one-word suffix
// and its qualifier form: "spaced" for " / ", "dash" for " - ", "paren" for a
// trailing parenthesis and "plain" for a last plain word. ok is false for
// any other shape, a suffix that is not one word of ASCII letters, or a base
// of fewer than two words.
func companySuffixParts(s string) (base []string, suffix, form string, ok bool) {
	q := splitCompanyQualifier(s)
	switch q.form {
	case "slash":
		if strings.Count(s, "/") != 1 || !strings.Contains(s, " / ") || strings.ContainsAny(s, "()") {
			return nil, "", "", false
		}
		form = "spaced"
	case "dash":
		if strings.Count(s, " - ") != 1 || strings.ContainsAny(s, "/()") {
			return nil, "", "", false
		}
		form = "dash"
	case "paren":
		if strings.ContainsAny(q.base, "/()") || strings.Contains(q.base, " - ") || strings.TrimSpace(s[strings.LastIndexByte(s, ')')+1:]) != "" {
			return nil, "", "", false
		}
		form = "paren"
	default:
		if strings.ContainsAny(s, "/()-_") {
			return nil, "", "", false
		}
		w := companyScoreWords(s, false)
		if len(w) != len(strings.Fields(s)) || len(w) < 3 {
			return nil, "", "", false
		}
		return w[:len(w)-1], w[len(w)-1], "plain", companyLetters(w[len(w)-1])
	}
	base = companyScoreWords(q.base, false)
	sw := companyScoreWords(q.suffix, false)
	if len(base) < 2 || len(sw) != 1 || len(strings.Fields(q.suffix)) != 1 || !companyLetters(sw[0]) {
		return nil, "", "", false
	}
	return base, sw[0], form, true
}

// companyLetters reports a non-empty word of lower-case ASCII letters.
func companyLetters(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < 'a' || s[i] > 'z' {
			return false
		}
	}
	return s != ""
}

// companySpacedAsDash rewrites a spaced slash suffix as a dash suffix when
// the pair has one base and two different one-word suffixes, no digit, and
// the other name is a spaced slash or a plain form.
//
// NATIVE company_name_fit/org.tsv R2_vow, R2_par, R4_sp and R7_sp (API 62.0
// and 67.0, both routes): "Orbel Trade / West" finds "Orbel Trade / East" at
// 93 and "Orbel Trade / East" finds "Orbel Trade / West" at 95, as across
// two dash forms, and suffixes of different lengths follow ("Orbel Trade /
// Art" finds "Orbel Trade / North" and "Orbel Trade / Outer"); "Orbel Trade
// East" finds "Orbel Trade / Mast" at 94, as it finds the dash form. A spaced
// slash against a dash form or an unspaced slash does not score as two dash
// forms, and a pair with one suffix keeps its own confidences. Against a
// parenthesis form a vowel candidate matches on some bases only ("Zorvex
// Freight", not "Orbel Trade" or "Mandolar Oil"); parenthesis forms and names
// with digits keep the earlier decision.
func companySpacedAsDash(a, b string) (string, string, bool) {
	if !strings.Contains(a, " / ") && !strings.Contains(b, " / ") || strings.ContainsAny(a, "0123456789()") || strings.ContainsAny(b, "0123456789()") {
		return a, b, false
	}
	ba, sa, fa, okA := companySuffixParts(a)
	bb, sb, fb, okB := companySuffixParts(b)
	if !okA || !okB || sa == sb || fa == "dash" || fb == "dash" || (fa != "spaced" && fb != "spaced") || strings.Join(ba, " ") != strings.Join(bb, " ") {
		return a, b, false
	}
	dash := func(s, form string) string {
		if form == "spaced" {
			return strings.Replace(s, " / ", " - ", 1)
		}
		return s
	}
	return dash(a, fa), dash(b, fb), true
}

// companyVowelSuffixScore scores a plain candidate against a dash form on
// one base when the candidate's one-word suffix starts with a vowel and the
// stored suffix differs and has the same length of four or more letters. The
// word scorer rejects every such pair. The caller tries it only when one name
// has " - ", the other has no hyphen or underscore, and neither has a digit.
//
// NATIVE company_name_fit/org.tsv R2_vow, R2_par, R3_long and R7_pl (API 62.0
// and 67.0, both routes), on "Orbel Trade", "Zorvex Freight", "Qa Trade" and
// six further bases: "East" finds "- West", "- Best" and "- Nest" at 93 and
// "- Wast", "- Mast" and "- Last" at 94, "Oast" finds "- West" and "- Mast";
// neither finds "- North" or "- South". "East" and "Oast" find "- Oast" and
// "- East" at 96, not "- Ease". R4_misc: "Art" does not find "- Bat".
//
// The dash candidate ("- East" and "West", native 91) matches on some bases
// and not on others (R7_pl: not on "Mandolar Oil", "Orbel Oil Outlet" or "Ab
// Logistics") and keeps the word scorer's decision, as a consonant candidate
// does.
func companyVowelSuffixScore(a, b string) (float64, bool) {
	ba, c, fa, okA := companySuffixParts(a)
	bb, s, fb, okB := companySuffixParts(b)
	if !okA || !okB || c == s || len(c) != len(s) || len(c) < 4 || !companyVowelStart(c) || !companyTwoWordBase(ba) || strings.Join(ba, " ") != strings.Join(bb, " ") {
		return 0, false
	}
	if fa != "plain" || fb != "dash" {
		return 0, false
	}
	switch {
	case c[1:] == s[1:] && companyVowelStart(s):
		return 96, true
	case companyVowelStart(s):
	case c[1:] == s[1:]:
		return 94, true
	case c[2:] == s[2:]:
		return 93, true
	}
	return 0, false
}

// companyTwoWordBase reports base words that are still two or more after one
// trailing legal designator is removed; a shorter base keeps the word
// scorer's decision. NATIVE company_name_fit/org.tsv R4_stem: a word and a
// designator are a one-word name ("Vexlets Inc" does not find "Vexlet Inc").
func companyTwoWordBase(base []string) bool {
	n := len(base)
	if n > 0 && companyDesignators[base[n-1]] {
		n--
	}
	return n >= 2
}

// companyVowelStart reports a word that starts with a, e, i, o or u.
func companyVowelStart(s string) bool {
	return s != "" && strings.IndexByte("aeiou", s[0]) >= 0
}

// companyVowelParenScore raises two parenthesis forms on one base whose
// one-word suffixes both start with a vowel and differ only in that letter
// to 100. The word scorer already matches them at 91.
//
// NATIVE company_name_fit/org.tsv s1_b (API 62.0 and 67.0, both routes, six
// bases): "Orbel Trade (East)", "(Oast)" and "(Uast)" match one another at
// 100 in every direction. Against a stored " - West" the same names match on
// some bases only; that verdict gate has no supported separator (s1_a, s1_c)
// and is not scored.
func companyVowelParenScore(a, b string) bool {
	if strings.ContainsAny(a, "0123456789/") || strings.ContainsAny(b, "0123456789/") {
		return false
	}
	ba, c, fa, okA := companySuffixParts(a)
	bb, s, fb, okB := companySuffixParts(b)
	return okA && okB && fa == "paren" && fb == "paren" && c != s && len(c) == len(s) && len(c) >= 4 &&
		companyVowelStart(c) && companyVowelStart(s) && c[1:] == s[1:] && companyTwoWordBase(ba) && strings.Join(ba, " ") == strings.Join(bb, " ")
}
