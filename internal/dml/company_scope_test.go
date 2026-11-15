package dml

import (
	"strings"
	"testing"
	"unicode"
)

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

func TestCompanyExactFastPathRetainsPriorDecision(t *testing.T) {
	for _, name := range []string{"Test Account", "Test Account 1", "Partner", " Partner ", "Partners", "", "Q & V", "Acme-1", "Élan Company"} {
		got, score := fuzzyCompany(name, name, false)
		base, baseScore := legacyFuzzyCompany(name, name, false)
		if got != base || score != baseScore {
			t.Errorf("%q: got %v %v, base %v %v", name, got, score, base, baseScore)
		}
	}
}

func TestCompanyFitShapePreservesClassification(t *testing.T) {
	prior := func(a, b string) bool {
		for _, s := range []string{a, b} {
			for _, r := range s {
				if unicode.IsDigit(r) {
					return true
				}
			}
			w := strings.Fields(strings.ReplaceAll(strings.ToLower(s), ".", ""))
			if len(w) > 1 && (w[len(w)-1] == "partner" || w[len(w)-1] == "sarl" || w[len(w)-1] == "bv") {
				return true
			}
		}
		return false
	}
	values := []string{"", ". Partner", "Acme Partner", "Acme p.a.r.t.n.e.r.", "Acme\tB.V. ", "partner", "Acme SARL", "Acme ١", "Test Account", "Test Account-1"}
	for _, a := range values {
		for _, b := range values {
			if got, want := companyFitShape(a, b), prior(a, b); got != want {
				t.Errorf("%q / %q: got %v, want %v", a, b, got, want)
			}
		}
	}
}
