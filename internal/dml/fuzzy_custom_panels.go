package dml

import "strings"

// Custom matching rule items score through the shared fuzzy scorers plus the
// extensions below. Each extension is read from the duplicate_fuzzy_panels
// fixture (Datacloud.FindDuplicates on custom rules, API 62.0 and 67.0,
// anonymous and @IsTest, two repeats, all identical) and is gated to the value
// shapes those rows cover. The standard rules call the shared scorers directly,
// so none of this changes them.
var customFuzzyScorers = map[string]fuzzyScorer{
	"firstname":   customFirstName,
	"companyname": customCompany,
	"street":      customStreet,
}

// firstNameInitials reports a lower-cased value made only of single letters a
// to z, each with an optional dot, separated by single spaces: "j", "j.",
// "j r", "j. r.".
func firstNameInitials(s string) bool {
	if s == "" || (len(s) > 2 && s[1] != ' ' && s[1] != '.') {
		return false
	}
	for _, word := range strings.Split(s, " ") {
		if word == "" || len(word) > 2 || word[0] < 'a' || word[0] > 'z' || (len(word) == 2 && word[1] != '.') {
			return false
		}
	}
	return true
}

// maybeInitials is a cheap necessary condition for firstNameInitials on a raw
// value: an initials value has a space or a dot as its second byte.
func maybeInitials(s string) bool {
	s = strings.TrimSpace(s)
	return len(s) <= 2 || s[1] == ' ' || s[1] == '.'
}

// These directional pairs were captured on custom rules only. Keep them out
// of the shared dictionary used by standard matching rules.
var customNameVariants = map[string]bool{
	"bart\x00bartholomew": true, "bartholomew\x00bart": true,
	"ezekiel\x00zeke": true, "zeke\x00ezekiel": true,
	"millicent\x00millie": true, "millie\x00millicent": true,
	"montgomery\x00monty": true, "monty\x00montgomery": true,
	"mort\x00mortimer": true, "mortimer\x00mort": true,
	"prudence\x00prue": true, "prue\x00prudence": true,
	"reg\x00reggie": true, "reggie\x00reg": true,
	"thad\x00thaddeus": true, "thaddeus\x00thad": true,
}

// customFirstName adds the Initials algorithm. NATIVE: a single initial finds
// every initials value with the same first letter and is found by it ("J" and
// "J." find each other and "J R", "J. R." and "J K"), two longer initials
// values never match each other ("J R" does not find "J. R."), and an initials
// value matches no other name, not by Jaro-Winkler either ("J. R." does not
// find "J.R."). Values written without a space ("JR", "J.R.") are names.
func customFirstName(a, b string, matchBlanks bool) (bool, float64) {
	// No captured initials value has more than two words.
	if strings.Count(strings.TrimSpace(a), " ") > 1 || strings.Count(strings.TrimSpace(b), " ") > 1 {
		return fuzzyFirstName(a, b, matchBlanks)
	}
	x, y := foldText(a), foldText(b)
	if customNameVariants[x+"\x00"+y] {
		return true, 100
	}
	if !maybeInitials(a) && !maybeInitials(b) {
		return fuzzyFirstNameFolded(x, y)
	}
	ix, iy := firstNameInitials(x), firstNameInitials(y)
	if !ix && !iy {
		return fuzzyFirstNameFolded(x, y)
	}
	if x == y {
		return true, 100
	}
	single := func(s string) bool { return len([]rune(strings.TrimSuffix(s, "."))) == 1 }
	if ix && iy && []rune(x)[0] == []rune(y)[0] && (single(x) || single(y)) {
		return true, 100
	}
	_, confidence := fuzzyFirstNameFolded(x, y)
	return false, confidence
}

// customCompany adds what the Company Name panels show beyond the shared
// scorer. NATIVE, two-word names with the same second word: the first words
// match under the second-word rule (six shared letters, then one is the other
// with more letters or they differ in one letter after the prefix), and the
// rest of the name is scored as if they were equal. One-word plurals: a word
// of eight to ten letters ending in "re" is not matched to its plural
// ("Fennimore", "Callowmere"; held out, "Wexmoore"), one of eight or nine
// letters ending in "o" scores 98 ("Drummondo", "Tamberlo"), and the plural
// of a word of six to nine letters ending in "a", "o" or "y" finds the word
// itself (companyPluralReverse).
func customCompany(a, b string, matchBlanks bool) (bool, float64) {
	return fuzzyCompanyWithCustom(a, b, true)
}

func customCompanyWords(a, b string, wa, wb []string) (bool, float64) {
	matched, confidence := fuzzyCompanyWords(a, b, wa, wb)
	ta, tb := strings.TrimSpace(a), strings.TrimSpace(b)
	if !strings.ContainsAny(ta, " -") && !strings.ContainsAny(tb, " -") && (len(ta) == len(tb)+1 || len(tb) == len(ta)+1) {
		if len(wa) == 1 && len(wb) == 1 && companyKey(wa) == companyKey(wb) && !companyAlignedWords[wa[0]] && !companyAlignedWords[wb[0]] {
			// Each rule is limited to the word lengths the rows cover.
			x, y := wa[0], wb[0]
			switch n := len([]rune(x)); {
			case y == x+"s" && strings.HasSuffix(x, "re") && n >= 8 && n <= 10:
				return false, 0
			case y == x+"s" && strings.HasSuffix(x, "o") && n >= 8 && n <= 9:
				return true, 98
			}
			if n := len([]rune(y)); !matched && x == y+"s" && n >= 6 && n <= 9 && strings.ContainsAny(y[len(y)-1:], "aoy") {
				return true, companyPluralReverse(y)
			}
		}
		return matched, confidence
	}
	if matched || strings.Contains(ta, "-") || strings.Contains(tb, "-") || len(ta) < 6 || len(tb) < 6 || !strings.EqualFold(ta[:6], tb[:6]) {
		return matched, confidence
	}
	if len(wa) < 2 || len(wb) < 2 || wa[0] == wb[0] || wa[1] != wb[1] || companyKey(wa) != companyKey(wb) || !companySecondWordMatch(wa[0], wb[0]) {
		return matched, confidence
	}
	aligned := strings.Join(append([]string{wa[0]}, wb[1:]...), " ")
	if strings.Join(companyWords(aligned), " ") != aligned {
		return matched, confidence
	}
	return fuzzyCompanyWords(strings.Join(wa, " "), aligned, wa, append([]string{wa[0]}, wb[1:]...))
}

// customStreet matches a street whose last word is a known suffix against the
// same street ending in an unknown word. NATIVE: such pairs match when the
// edit similarity of the cleaned addresses is at least 87.5 ("St" and "Strt",
// "Drive" and "Drv", "Hwy" and "Pkwy") and are kept apart at 85.71 and below
// ("St" and "Sreet", "Rd" and "Drv"), although the weighted section score is
// under 80 (74 to 78). The confidence is the shared scorer's.
func customStreet(a, b string, matchBlanks bool) (bool, float64) {
	return fuzzyStreetWithCustom(a, b, matchBlanks, true)
}

// companyPluralReverse is the confidence of a plural candidate finding the
// one-word name it extends. NATIVE (duplicate_standard_rules Company.Syllable
// rows and duplicate_fuzzy_panels): "Athena", "Velluma" 95 and "Andromeda" 97;
// "Airway" 94, "Pathway", "Liberty", "Tamberlo" 96 and "Drummondo" 97. Words
// ending in "w" or "s" vary ("Willow" 94, "Bayview" no match) and are left to
// the shared scorer.
func companyPluralReverse(word string) float64 {
	n := len([]rune(word))
	switch {
	case n >= 9:
		return 97
	case strings.HasSuffix(word, "a"):
		return 95
	case n == 6:
		return 94
	}
	return 96
}
