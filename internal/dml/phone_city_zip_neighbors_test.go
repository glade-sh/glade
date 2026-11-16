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
