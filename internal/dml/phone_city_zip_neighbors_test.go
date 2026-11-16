package dml

import "testing"

// Uncaptured neighbors of the captured City and ZIP shapes keep the
// origin/main verdicts and confidences: white space other than ASCII spaces,
// non-ASCII text, other ZIP separators, and spaced or longer extensions.
func TestPhoneCityZipUncapturedNeighborsKeepBase(t *testing.T) {
	for _, tc := range []struct {
		method, a, b string
		match        bool
		score        float64
	}{
		{"City", "Los\tAngeles", "Los Angeles", true, 100},
		{"City", "Los \tAngeles", "Los Angeles", true, 100},
		{"City", "São  Paulo", "São Paulo", true, 100},
		{"City", "Los  Angeles", "Los\tAngeles", true, 100},
		{"City", "Paris  Nord", "París Nord", true, 95},
		{"Zip", "12345_6789", "12345", false, 0},
		{"Zip", "12345,6789", "12345", false, 0},
		{"Zip", "12345:6789", "12345-6789", false, 0},
	} {
		match, score := FuzzyScore(tc.method, tc.a, tc.b, false)
		if match != tc.match || score != tc.score {
			t.Errorf("%s %q/%q: (%v,%v), base (%v,%v)", tc.method, tc.a, tc.b, match, score, tc.match, tc.score)
		}
	}
	for _, tc := range []struct {
		a, b  string
		match bool
		score float64
	}{
		{"12345 -678", "12345", false, 100},
		{"12345 - 6789", "12345", false, 100},
		{"12345-123456", "12345", false, 100},
		{"12345  -6789", "12345-6789", false, 100},
		{"SW1A\t1AA", "SW1A 1AA", true, 100},
		{"SW1A  1AA", "SW1A\t1AA", true, 100},
		{"STRASSE 1", "straße 1", false, 100},
		{"Å1A  1A1", "å1a 1a1", true, 100},
	} {
		if match, score := standardZip(tc.a, tc.b); match != tc.match || score != tc.score {
			t.Errorf("standard %q/%q: (%v,%v), base (%v,%v)", tc.a, tc.b, match, score, tc.match, tc.score)
		}
	}
}

// The raw City confidence an unmatched item adds inside AND and OR groups
// compares the same text as the verdict: between two plain ASCII values the
// inner spaces count as typed (N5.cm1: "LosAngeles" does not find
// "Los  Angeles", edit similarity 83.3), and any other pair keeps the folded
// text (those pairs match, so only their raw score is pinned here).
func TestCityRawConfidenceFollowsVerdictSpacing(t *testing.T) {
	for _, tc := range []struct {
		a, b    string
		want    float64
		matched bool
	}{
		{"LosAngeles", "Los  Angeles", averageWith100(100 * (1 - 2.0/12)), false},
		{"Los Angles", "Los  Angeles", averageWith100(100 * (1 - 2.0/12)), false},
		{"LosAngeles", "Los Angeles", averageWith100(100 * (1 - 1.0/11)), true},
		{"LosAngeles", "Los\tAngeles", averageWith100(100 * (1 - 1.0/11)), true},
	} {
		if got := FuzzyRawConfidence("City", tc.a, tc.b, false); got != tc.want {
			t.Errorf("%q/%q: %v, want %v", tc.a, tc.b, got, tc.want)
		}
		if matched, _ := FuzzyScore("City", tc.a, tc.b, false); matched != tc.matched {
			t.Errorf("%q/%q: matched %v, want %v", tc.a, tc.b, matched, tc.matched)
		}
	}
}
