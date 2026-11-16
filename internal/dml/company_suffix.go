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
// the pair has one base and two different one-word suffixes, and the other
// name is a spaced slash, plain or parenthesis form.
//
// NATIVE company_name_fit/org.tsv R2_vow and R2_par (API 62.0 and 67.0, both
// routes): every such pair scores what the same pair with " - " in place of
// " / " scores. "Orbel Trade / West" finds "Orbel Trade / East" at 93 and
// "Orbel Trade / East" finds "Orbel Trade / West" at 95, as across two dash
// forms; "Orbel Trade / East" finds "Orbel Trade West" at 91 and "Orbel
// Trade East" finds "Orbel Trade / Mast" at 94, as with the dash forms; and
// "Zorvex Freight / East" against "Zorvex Freight (West)" scores and decides
// as "Zorvex Freight - East" does. A spaced slash against a dash form or an
// unspaced slash does not score as two dash forms, and a pair with one
// suffix keeps its own confidences.
func companySpacedAsDash(a, b string) (string, string, bool) {
	if !strings.Contains(a, " / ") && !strings.Contains(b, " / ") {
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

// companyVowelSuffixScore scores a dash form against a plain form on one
// base when the candidate's one-word suffix starts with a vowel and the
// stored suffix differs and has the same length. The word scorer rejects
// every such pair. The caller tries it only when one name has " - ", the
// other has no hyphen or underscore, and neither has a digit.
//
// NATIVE company_name_fit/org.tsv R2_vow, R2_par and R3_long (API 62.0 and
// 67.0, both routes), on "Orbel Trade", "Zorvex Freight" and "Qa Trade":
//   - Dash candidate: "- East" finds "West", "Best", "Nest" and "Mast" at 91,
//     "- Oast" finds "Mast" but not "West"; "- Ease" finds neither; "- East"
//     and "- Oast" find "Oast" and "East" at 94.
//   - Plain candidate: "East" finds "- West", "- Best" and "- Nest" at 93 and
//     "- Wast", "- Mast" and "- Last" at 94, "Oast" finds "- West" and "-
//     Mast"; neither finds "- North" or "- South". "East" and "Oast" find
//     "- Oast" and "- East" at 96, not "- Ease".
//
// A consonant candidate keeps the word scorer's decision.
func companyVowelSuffixScore(a, b string) (float64, bool) {
	ba, c, fa, okA := companySuffixParts(a)
	bb, s, fb, okB := companySuffixParts(b)
	if !okA || !okB || c == s || len(c) != len(s) || !companyVowelStart(c) || strings.Join(ba, " ") != strings.Join(bb, " ") {
		return 0, false
	}
	switch {
	case fa == "dash" && fb == "plain":
		if companyVowelStart(s) {
			if c[1:] == s[1:] {
				return 94, true
			}
			return 0, false
		}
		if jaroWinkler(c, s) >= .8 {
			return 91, true
		}
	case fa == "plain" && fb == "dash":
		switch {
		case c[1:] == s[1:] && companyVowelStart(s):
			return 96, true
		case companyVowelStart(s):
		case c[1:] == s[1:]:
			return 94, true
		case c[2:] == s[2:]:
			return 93, true
		}
	}
	return 0, false
}

// companyVowelStart reports a word that starts with a, e, i, o or u.
func companyVowelStart(s string) bool {
	return s != "" && strings.IndexByte("aeiou", s[0]) >= 0
}

// companyVowelParenFinds reports whether a candidate suffix that starts with
// a vowel finds a different stored suffix of the same length across a dash
// and a parenthesis form.
//
// NATIVE company_name_fit/org.tsv R1_dir, R2_par, R3_base and R3_long (API
// 62.0 and 67.0, both routes): a suffix that differs only in its first letter
// matches on every base ("Orbel Trade (East)" finds "Orbel Trade - Wast",
// "Zorvex Freight - Oast" finds "Zorvex Freight (Mast)" and "(East)"). Other
// consonant suffixes match by the syllable alignment of the whole names: at
// 94.57 or more they match ("(East)" and "- West" on "Zorvex Fret", "Orbel
// Tradex", "Orbel Trading", "Orbelvanto Trade" and "Zorvex Freight"), at
// 93.90 or less they do not ("Orbel Trade", "Qa Freight", "Qa Trade", and
// "(Oast)" and "- Best" on "Zorvex Freight"). The threshold of 94 is fitted
// between those values; "Zorvex Freightways (Oast)" finds "- Best" natively
// at 93.97 and stays unmatched.
func companyVowelParenFinds(candidate, stored, a, b string) bool {
	if len(candidate) != len(stored) || !companyVowelStart(candidate) {
		return false
	}
	if candidate[1:] == stored[1:] {
		return true
	}
	if companyVowelStart(stored) {
		return false
	}
	return companySAPSRaw(companyScoreWords(a, false), companyScoreWords(b, false)) >= 94
}
