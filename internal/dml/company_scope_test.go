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
	values := []string{"", ". Partner", "Acme Partner", "Acme p.a.r.t.n.e.r.", "Acme\tB.V. ", "partner", "Acme SARL", "Acme ١", "Test Account", "Test Account-1", "Acme\u00a0SARL", "Acme\u2003Partner"}
	for _, a := range values {
		for _, b := range values {
			if got, want := companyFitShape(a, b), prior(a, b); got != want {
				t.Errorf("%q / %q: got %v, want %v", a, b, got, want)
			}
		}
	}
}

func TestCompanyFitKeysMatchWordKey(t *testing.T) {
	values := []string{"", " ", ".", "&", "& &", "The", "The The", "the and", "A of the", "AT&T", "Q & V", "Q&V", "Barnes&Noble",
		"Acme L.L.C.", "Acme L L C", "Acme Pvt Ltd", "Acme Private Limited", "Acme S.A.", "Acme B.V.", "Acme Incorporated Company",
		"Acme Inc Inc Inc", "Acme Partner", "Acme SARL", "Test Account-1", "Test Account_2", "Qelthorin Freight - West",
		"Qelthorin Freight/West", "Qelthorin Freight (West)", "Zevrula, Inc.", "The Zevrula Company", "Department of Energy",
		"St Zivra Hospital", "U.S. Zivra", "7-Zorval", "ZQMorvella", "Bright-Path Solutions Inc", "Hewlett-Packard Enterprise",
		"Élan Company", "Acme ١", "one two three four five six seven eight nine ten eleven twelve thirteen fourteen fifteen sixteen seventeen",
		strings.Repeat("&", 60), strings.Repeat("a&", 40), "Company 5001 Okafor Works", "Okafor-Ashton Group"}
	for _, s := range values {
		var buf [companyKeyBuffer]byte
		key, words, ok := companyFitKeyASCII(s, buf[:0])
		if !ok {
			continue
		}
		w := companyScoreWords(s, true)
		if string(key) != companyFitKeyOf(w) || words != len(w) {
			t.Errorf("%q: got %q %d, want %q %d", s, key, words, companyFitKeyOf(w), len(w))
		}
	}
	for _, a := range values {
		for _, b := range values {
			same, na, nb := companyFitKeys(a, b)
			if want := companyFitKey(a) == companyFitKey(b); same != want || na != len(companyScoreWords(a, true)) || nb != len(companyScoreWords(b, true)) {
				t.Errorf("%q / %q: got %v %d %d, want %v", a, b, same, na, nb, want)
			}
		}
	}
}

// The captured numbered, designator and qualifier pairs share their leading
// words and their qualifier. NATIVE company_name_fit/org.tsv B3_cw1 and
// B1_co3 reject the nearest pairs without a digit ("Qelthorin Freights" and
// "Qelthorin Freighter", "Testing Account" and "Tester Account", "Qelthorin
// Freight Systems" and "Qelthorin Freight Services Group", a " - " suffix and
// a different last word), so these uncaptured pairs keep the word scorer's
// decision.
func TestCompanyFitKeepsWordScorerOutsideCapturedShapes(t *testing.T) {
	for _, pair := range [][2]string{
		{"Qelthorin Freights 1", "Qelthorin Freighter 1"},
		{"Qelthorin Freights", "Qelthorin Freighter 2"},
		{"Internal Systems 1", "Internet Systems 1"},
		{"Northwest Airlines 2", "Northwestern Airliners 2"},
		{"Acme Partners 1", "Acme Partner 1"},
		{"Qelthorin Freight Systems 1", "Qelthorin Freight Services Group"},
		{"Zorvex Logistics 12", "Zorvex Logistics 12 Northern Division"},
		{"Zorvex Marine SARL", "Zorvex Mariner BV"},
		{"Zorvex Freight - North", "Zorvex Freight/West"},
		{"Zorvex Freight (East)", "Zorvex Freight - North"},
		{"Zorvex Region 1 West", "Zorvex Region 2 East"},
	} {
		for _, p := range [][2]string{pair, {pair[1], pair[0]}} {
			got, score := fuzzyCompany(p[0], p[1], false)
			base, baseScore := legacyFuzzyCompany(p[0], p[1], false)
			if got != base || got && score != baseScore {
				t.Errorf("%q -> %q: got %v %v, word scorer %v %v", p[0], p[1], got, score, base, baseScore)
			}
		}
	}
	// The same routes still match pairs with the captured shape.
	for _, pair := range [][2]string{
		{"Qelthorin Freights", "Qelthorin Freights 1"},
		{"Zorvex Region 1 Sales", "Zorvex Region 2 Sales"},
		{"Test Account1", "Test Account 1"},
		{"7-Zorval", "7 Zorval"},
		{"Zorvex Freight - West", "Zorvex Freight/West"},
	} {
		if got, _ := fuzzyCompany(pair[0], pair[1], false); !got {
			t.Errorf("%q -> %q does not match", pair[0], pair[1])
		}
	}
}

func TestCompanyUnnumbered(t *testing.T) {
	for in, want := range map[string]string{
		"Test Account 1": "Test Account", "Test Account-1": "Test Account", "Test Account_2": "Test Account", "TestAccount1": "TestAccount",
		"Test Account #1": "Test Account", "1 Test Account": "Test Account", "Test 1 Account": "Test Account", "7-Zorval": "Zorval",
		"3M Company": "3M Company", "Web2Go Systems": "Web2Go Systems", "001": "", "Acme": "Acme",
	} {
		if got := companyUnnumbered(in); got != want {
			t.Errorf("%q: got %q, want %q", in, got, want)
		}
	}
}
