package dml

import "strings"

// companySlashScore scores the qualifier pairs the main scorer leaves to the
// word scorer: two slash forms, a spaced slash against a plain or slash name,
// and two parenthesis forms with one extra base word. It is tried only when
// the other routes reject the pair.
//
// NATIVE company_name_fit/org.tsv D1_qf (API 62.0 and 67.0, both routes):
//   - "Zorvex Freight/West" and "Zorvex Freight/East" match at 100 in both
//     directions, "Zorvex Freights/West" and "Zorvex Freight/West" at 100, and
//     "Zorvex Freight/East" and "Zorvex Freights/West" at 92 (the base and
//     the suffix both differ). A suffix with a different number of words
//     scores 91.
//   - "Zorvex Freight / West" (spaced) matches "Zorvex Freight West" at 96,
//     "Zorvex Freight/West" at 96, and each against the spaced name at 98;
//     with "Freights" as the base the pair scores 94 and 91.
//   - "Zorvex Freight Services (West)" and "Zorvex Freight (West)" match at 93
//     whatever the suffix.
func companySlashScore(a, b string) (float64, bool) {
	if !strings.ContainsAny(a, "/(") && !strings.ContainsAny(b, "/(") {
		return 0, false
	}
	x, y := splitCompanyQualifier(a), splitCompanyQualifier(b)
	words := func(s string) []string { return companyScoreWords(s, false) }
	bx, by := words(x.base), words(y.base)
	switch {
	case x.form == "paren" && y.form == "paren":
		short, long := bx, by
		if len(short) > len(long) {
			short, long = long, short
		}
		if len(long) == len(short)+1 && strings.Join(long[:len(short)], " ") == strings.Join(short, " ") && companySuffixStart(x.suffix, y.suffix) {
			return 93, true
		}
		return 0, false
	case x.form == "slash" && y.form == "slash":
	case x.form == "slash" && y.form == "plain", x.form == "plain" && y.form == "slash":
		p, q := x, y
		if p.form != "plain" {
			p, q = q, p
		}
		if !strings.Contains(q.base+q.suffix, " ") && !strings.Contains(a+b, " / ") {
			return 0, false
		}
		w := words(p.base)
		if len(w) < 2 || strings.Join(w[:len(w)-1], " ") != strings.Join(words(q.base), " ") || w[len(w)-1] != strings.Join(words(q.suffix), " ") {
			return 0, false
		}
		if x.form == "plain" {
			return 98, true
		}
		return 96, true
	default:
		return 0, false
	}
	spacedA, spacedB := strings.Contains(a, " / "), strings.Contains(b, " / ")
	if len(bx) != len(by) {
		return 0, false
	}
	equalBase := strings.Join(bx, " ") == strings.Join(by, " ")
	for i := range bx {
		if !strings.HasPrefix(bx[i], by[i]) && !strings.HasPrefix(by[i], bx[i]) {
			return 0, false
		}
	}
	sx, sy := words(x.suffix), words(y.suffix)
	equalSuffix := strings.Join(sx, " ") == strings.Join(sy, " ")
	if !equalSuffix && (len(sx) == 0 || len(sy) == 0 || jaroWinkler(sx[0], sy[0]) < .8) {
		// "West" and "East" match; "West" and "North" do not.
		return 0, false
	}
	switch {
	case spacedA != spacedB:
		switch {
		case equalSuffix && equalBase:
			if spacedA {
				return 96, true
			}
			return 98, true
		case equalSuffix:
			if spacedA {
				return 94, true
			}
			return 91, true
		case equalBase && !spacedA && len(sx) == len(sy):
			return 93, true
		}
		return 0, false
	case spacedA:
		return 0, false
	case len(sx) != len(sy):
		return 91, true
	case equalBase || equalSuffix:
		return 100, true
	}
	// Only the shorter base finds the longer: "Freight/East" finds
	// "Freights/West" at 92 and "Freights/West" does not find "Freight/East".
	for i := range bx {
		if !strings.HasPrefix(by[i], bx[i]) {
			return 0, false
		}
	}
	return 92, true
}

// companySuffixStart reports that the first words of two suffixes are equal
// or one starts the other ("West" and "West Coast").
func companySuffixStart(a, b string) bool {
	x, y := companyScoreWords(a, false), companyScoreWords(b, false)
	return len(x) > 0 && len(y) > 0 && (strings.HasPrefix(x[0], y[0]) || strings.HasPrefix(y[0], x[0]))
}
