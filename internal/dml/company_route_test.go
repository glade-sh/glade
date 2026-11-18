package dml

import "testing"

// The fitted scorer comes from custom Company Name rules. The standard
// Account rule is a different profile, so its route keeps the word scorer.
func TestCompanyStandardRouteKeepsWordScorer(t *testing.T) {
	for _, pair := range [][2]string{
		{"Test Account", "Test Account 1"},
		{"Test Account 1", "Test Account"},
		{"Account1", "Account2"},
		{"Qelthorin-Freight", "Qelthorin Freights"},
		{"Qelthorin Freight/West", "Qelthorin Freight - West"},
		{"Zevrula SARL", "Zevrula"},
	} {
		got, score := standardCompany(pair[0], pair[1])
		want, wantScore := false, 0.0
		if standardCompanyGate(pair[0], pair[1]) {
			want, wantScore = legacyFuzzyCompany(pair[0], pair[1], false)
		}
		if got != want || score != wantScore {
			t.Errorf("%q -> %q: got %v %v, want %v %v", pair[0], pair[1], got, score, want, wantScore)
		}
	}
}

// NATIVE company_name_fit/org.tsv:72 (B3_cw1) and
// duplicate_fuzzy_matching/org.tsv:25,75,108. A hyphen joins two words. The
// joined form is scored when its side has no more words than the other;
// otherwise the prior decision stands.
func TestCompanyHyphenJoinRoute(t *testing.T) {
	for _, c := range []struct {
		candidate, stored string
		match             bool
	}{
		{"Qelthorin-Freight", "Qelthorin Freights", true},
		{"Qelthorin Freights", "Qelthorin-Freight", true},
		{"Qelthorin-Freight", "Qelthorin Freighter", true},
		{"Qelthorin-Freight", "Qelthorin Freight East", true},
		{"Harley-Davidson", "Harley Davidsan", true},
		{"Harley Davidsan", "Harley-Davidson", true},
		{"Qelthorin Freighter", "Qelthorin-Freight", false},
		{"Qelthorin Freight East", "Qelthorin-Freight", false},
		{"Qelthorin-Freight", "Qelthorin Freight West", false},
		{"Qelthorin-Freight", "Qelthorin Freight Services", false},
		{"Hewlett-Packard", "Hewlett Packard Enterprise", false},
		{"Dunder-Mifflin Paper", "Dunder Mifflin", false},
		{"Dunder Mifflin", "Dunder-Mifflin Paper", false},
	} {
		if got, _ := fuzzyCompany(c.candidate, c.stored, false); got != c.match {
			t.Errorf("%q -> %q: got %v, want %v", c.candidate, c.stored, got, c.match)
		}
	}
}

// NATIVE company_name_fit/org.tsv:72 (B3_cw1). A " - " suffix against a
// slash suffix with the same base is scored by the qualifier table.
func TestCompanyDashQualifierRoute(t *testing.T) {
	for _, c := range []struct {
		candidate, stored string
		score             float64
	}{
		{"Qelthorin Freight - West", "Qelthorin Freight/West", 95},
		{"Qelthorin Freight/West", "Qelthorin Freight - West", 97},
		{"Qelthorin Freight (West)", "Qelthorin Freight - West", 96},
		{"Qelthorin Freight - West", "Qelthorin Freight (West)", 96},
	} {
		if got, score := fuzzyCompany(c.candidate, c.stored, false); !got || score != c.score {
			t.Errorf("%q -> %q: got %v %v, want match %v", c.candidate, c.stored, got, score, c.score)
		}
	}
	for _, pair := range [][2]string{
		{"Qelthorin Freight Services", "Qelthorin Freight - West"},
		{"Qelthorin Freights", "Qelthorin Freight - West"},
		{"Qelthorin Freight", "Qelthorin Freight - West"},
	} {
		if got, _ := fuzzyCompany(pair[0], pair[1], false); got {
			t.Errorf("%q -> %q matches; native does not", pair[0], pair[1])
		}
	}
}
