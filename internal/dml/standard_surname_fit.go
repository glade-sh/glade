package dml

import "strings"

// standardSurnameFit is the Last Name item of the standard Contact and Lead
// rules where the phonetic key and a raw keyboard score decide (the surname
// panels Standard.Field.Contact.Last and Standard.Field.Lead.Last). It adds
// matches the earlier surname paths leave out and never removes one.
//
// The names are folded (lower case, hyphens and apostrophes deleted, spaces
// kept). Accents stay: an accented letter has no key, so two names that differ
// at an accent match only through an earlier path ("García" never finds
// "Gracia"; the Metaphone 3 code of the accented name differs too). Names longer than nine characters never match.
// Equal, non-empty Metaphone 3 codes with equal space and hyphen counts score 100.
// Otherwise the names need one first letter and a raw keyboard score at or
// above a floor: 93 for two names of one length (two or more letters), 91
// for two names that differ by one trailing letter (shorter name two to eight
// letters). The item is the raw score; the rule confidence is the mean of the
// item with two exact items, (200 + item) / 3.
func standardSurnameFit(rawA, rawB string) (item float64, ok bool) {
	a, b := standardSurnameFold(rawA), standardSurnameFold(rawB)
	if a == b {
		return 100, true
	}
	if len(a) > 9 || len(b) > 9 {
		return 0, false
	}
	if code := lastNameMetaphone3(a); code != "" && code == lastNameMetaphone3(b) &&
		strings.Count(rawA, " ") == strings.Count(rawB, " ") &&
		strings.Count(rawA, "-") == strings.Count(rawB, "-") {
		return 100, true
	}
	var floor float64
	switch short, long := min(len(a), len(b)), max(len(a), len(b)); {
	case len(a) == len(b):
		if len(a) < 2 {
			return 0, false
		}
		floor = 93
	case short >= 2 && short <= 8 && long == short+1:
		floor = 91
	default:
		return 0, false
	}
	if a == "" || a[0] != b[0] {
		return 0, false
	}
	score, ok := standardSurnameKeyboard(a, b)
	if !ok || score < floor {
		return 0, false
	}
	return score, true
}

func standardSurnameFold(s string) string {
	return strings.NewReplacer("-", "", "'", "").Replace(strings.ToLower(s))
}

// standardSurnameKeyboard compares two folded names position by position over
// the shorter name. A differing position needs two letter keys: the key
// distance is the column distance, plus two between the top and the bottom
// row, and costs 0.05 up to one step and 0.08 for every step beyond the
// first. Each trailing letter costs 0.25. The score is 100 * (1 - cost / the
// longer length).
func standardSurnameKeyboard(a, b string) (float64, bool) {
	if len(a) > len(b) {
		a, b = b, a
	}
	cost := 0.0
	for i := 0; i < len(a); i++ {
		if a[i] == b[i] {
			continue
		}
		px, okx := qwertyPosition[rune(a[i])]
		py, oky := qwertyPosition[rune(b[i])]
		if !okx || !oky || a[i] >= 0x80 || b[i] >= 0x80 {
			return 0, false
		}
		steps := abs(px[1] - py[1])
		if abs(px[0]-py[0]) == 2 {
			steps += 2
		}
		if steps <= 1 {
			cost += 0.05
		} else {
			cost += 0.08 * float64(steps-1)
		}
	}
	cost += 0.25 * float64(len(b)-len(a))
	return 100 * (1 - cost/float64(len(b))), true
}
