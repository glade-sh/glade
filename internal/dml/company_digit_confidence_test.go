package dml

import "testing"

// NATIVE company_name_fit/org.tsv:4 (B1_co1_a) and :12 (B1_co1_b), istest 62.0.
// Digits joined to the same multi-word base score 100 in both directions, and
// a hyphen or underscore digit join against a single letter scores 91.
func TestCompanyDigitJoinConfidence(t *testing.T) {
	for _, c := range []struct {
		candidate, stored string
		confidence        float64
	}{
		{"Test Account1", "Test Account2", 100},
		{"Test Account2", "Test Account1", 100},
		{"Test Account_2", "Test Account B", 91},
		{"Test Account-1", "Test Account B", 91},
		{"Test Account B", "Test Account_2", 91},
		{"Test Account B", "Test Account-1", 91},
	} {
		got, score := fuzzyCompany(c.candidate, c.stored, false)
		if !got || score != c.confidence {
			t.Errorf("%q -> %q: got %v %v, want true %v", c.candidate, c.stored, got, score, c.confidence)
		}
	}
}
