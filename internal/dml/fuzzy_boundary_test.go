package dml

import (
	"math"
	"testing"
)

func TestCustomFuzzyBoundaries(t *testing.T) {
	for _, c := range []struct {
		method, a, b string
		match        bool
		score        float64
	}{
		{"FirstName", "J", "J R S", false, 88},
		{"LastName", "Kellingsworth", "Kellingswokwr", true, 97},
		{"LastName", "Fallojbrihse", "Falloprcidbe", true, 95},
	} {
		m, s := FuzzyScore(c.method, c.a, c.b, false)
		if m != c.match || math.Floor(s+0.5) != c.score {
			t.Errorf("%s %q/%q: got %v %.4f; want %v %.0f", c.method, c.a, c.b, m, s, c.match, c.score)
		}
	}
}

func BenchmarkCustomFuzzyPanel(b *testing.B) {
	cases := [][3]string{
		{"FirstName", "J", "J. R."}, {"FirstName", "Bart", "Bartholomew"},
		{"LastName", "Kellingsworth", "Kellingswokwr"}, {"LastName", "Fallojbrihse", "Falloprcidbe"},
		{"CompanyName", "Brightwater Logistics", "Brightwaters Logistics"}, {"CompanyName", "Drummondos", "Drummondo"},
		{"Street", "418 Larkwhistle St", "418 Larkwhistle Strt"}, {"Street", "72 Dunholt Drive", "72 Dunholt Drv"},
	}
	for i := 0; i < b.N; i++ {
		for _, c := range cases {
			FuzzyScore(c[0], c[1], c[2], false)
		}
	}
}
