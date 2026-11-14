package dml

import (
	"math"
	"strings"
	"testing"
)

func TestCustomFuzzyBoundaries(t *testing.T) {
	for _, c := range []struct {
		method, a, b string
		match        bool
		score        float64
	}{
		{"FirstName", "J", "J R S", false, 88},
	} {
		t.Run(c.method+"/"+c.a, func(t *testing.T) {
			m, s := FuzzyScore(c.method, c.a, c.b, false)
			if m != c.match || math.Floor(s+0.5) != c.score {
				t.Errorf("%s %q/%q: got %v %.4f; want %v %.0f", c.method, c.a, c.b, m, s, c.match, c.score)
			}
		})
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

// These native matches remain a known gap until their confidence is supported.
func TestCustomSurnameKeepsBaseline(t *testing.T) {
	for _, p := range [][2]string{{"Kellingsworth", "Kellingswokwr"}, {"Fallojbrihse", "Falloprcidbe"}, {"Brandenstock", "Brandepswohk"}} {
		got, score := FuzzyScore("LastName", p[0], p[1], false)
		want, baseline := fuzzyLastName(p[0], p[1], false)
		if got != want || score != baseline {
			t.Errorf("%q/%q differs from base", p[0], p[1])
		}
	}
}

func TestFoldTextPreservesBaseline(t *testing.T) {
	inputs := []string{"", "  Mixed  Case\tWords ", "\u00a0É X\u2003", "Alpha Beta", "a  b"}
	for c := 0; c < 256; c++ {
		inputs = append(inputs, "A"+string(rune(c))+" B")
	}
	for _, input := range inputs {
		want := strings.Join(strings.Fields(strings.ToLower(input)), " ")
		if got := foldText(input); got != want {
			t.Errorf("foldText(%q) = %q; want %q", input, got, want)
		}
	}
}
