package dml

import "testing"

// The short-ampersand fit rows and the earlier duplicate-rule rows disagree
// on the generalized abbreviation shape. Keep the earlier decision until a
// captured distinction is available. See company_name_fit/org.tsv:68 and
// duplicate_fuzzy_matching/org.tsv:90,431.
func TestCompanyShortAmpersandRetainsEarlierDecision(t *testing.T) {
	for _, pair := range [][2]string{{"Q&V", "QV"}, {"QV", "Q&V"}, {"Q & V", "QV"}, {"QV", "Q and V"}} {
		got, score := fuzzyCompany(pair[0], pair[1], false)
		base, baseScore := legacyFuzzyCompany(pair[0], pair[1], false)
		if got != base || score != baseScore || got {
			t.Errorf("%q -> %q: got %v %v, base %v %v", pair[0], pair[1], got, score, base, baseScore)
		}
	}
}
