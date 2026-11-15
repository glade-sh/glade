package dml

import (
	"strings"
	"testing"
)

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

func TestEditDistanceKeepsBaseline(t *testing.T) {
	baseline := func(a, b []rune) int {
		prev, cur := make([]int, len(b)+1), make([]int, len(b)+1)
		for j := range prev {
			prev[j] = j
		}
		for i := 1; i <= len(a); i++ {
			cur[0] = i
			for j := 1; j <= len(b); j++ {
				cost := 0
				if a[i-1] != b[j-1] {
					cost = 1
				}
				cur[j] = min(prev[j]+1, cur[j-1]+1, prev[j-1]+cost)
			}
			prev, cur = cur, prev
		}
		return prev[len(b)]
	}
	values := []string{"", "a", "b", "abc", "axc", "éclair", "éclat", strings.Repeat("a", 63) + "x", strings.Repeat("a", 64) + "y", strings.Repeat("b", 70)}
	for _, a := range values {
		for _, b := range values {
			got, want := levenshtein([]rune(a), []rune(b)), baseline([]rune(a), []rune(b))
			if got != want {
				t.Errorf("distance %q/%q = %d, want %d", a, b, got, want)
			}
		}
	}
}

// The panel records 96 for the reordered six-word titles in both directions.
func TestReorderedSixWordTitleNativeConfidence(t *testing.T) {
	for _, p := range [][2]string{
		{"Deputy Global Principal Retail Brand Strategist", "Strategist Global Principal Retail Brand"},
		{"Strategist Global Principal Retail Brand", "Deputy Global Principal Retail Brand Strategist"},
	} {
		m, score := FuzzyScore("Title", p[0], p[1], false)
		if !m || score != 96 {
			t.Errorf("title: got %v %.0f; native 96", m, score)
		}
	}
}

func TestTitleUncapturedNeighborsKeepBaseline(t *testing.T) {
	for _, tc := range []struct {
		a, b  string
		match bool
		score float64
	}{
		{"Strat", "Global Principal Retail Brand Strategist", false, 0},
		{"Global Principal Retail Brand Strategist", "Strat", false, 0},
		{"Alpha Bravo Charlie Delta Echo Foxtrot", "Alpha Bravo Charlie Golf Hotel India", true, 94},
		{"Alpha Bravo Charlie Golf Hotel India", "Alpha Bravo Charlie Delta Echo Foxtrot", true, 94},
	} {
		got, score := FuzzyScore("Title", tc.a, tc.b, false)
		if got != tc.match || score != tc.score {
			t.Errorf("%q / %q: got %v %v; baseline %v %v", tc.a, tc.b, got, score, tc.match, tc.score)
		}
	}
}

// Panel t1 pair 5a and a5, at both APIs and routes.
func TestSixWordTitlePartialOverlapNativeConfidence(t *testing.T) {
	for _, p := range [][2]string{
		{"Deputy Global Principal Retail Brand Strategist", "Brand Strategist Lead"},
		{"Brand Strategist Lead", "Deputy Global Principal Retail Brand Strategist"},
	} {
		match, score := FuzzyScore("Title", p[0], p[1], false)
		if !match || score != 89 {
			t.Errorf("%q / %q: got %v %v; native true 89", p[0], p[1], match, score)
		}
	}
}

// The street rows hold one-word names. A longer name keeps the shared scorer.
func TestCustomStreetMultiwordNameKeepsBaseline(t *testing.T) {
	for _, p := range [][2]string{
		{"418 larkwhistle oak St", "418 larkwhistle oak Sreet"},
		{"418 larkwhistle oak St", "418 larkwhistle oak Strt"},
		{"418 Larkwhistle Oak Strt", "418 Larkwhistle Oak St"},
		{"72 Dunholt Glen Drive", "72 Dunholt Glen Drv"},
	} {
		got, score := FuzzyScore("Street", p[0], p[1], false)
		want, baseline := fuzzyStreet(p[0], p[1], false)
		if got != want || score != baseline {
			t.Errorf("%q / %q: got %v %v; shared scorer %v %v", p[0], p[1], got, score, want, baseline)
		}
	}
	// The one-word rows still match.
	if got, _ := FuzzyScore("Street", "418 Larkwhistle St", "418 Larkwhistle Strt", false); !got {
		t.Error("the captured one-word street pair no longer matches")
	}
}

// The title rows hold no stop word and repeat no word. These values were read
// from the scorer on origin/main.
func TestTitleRepeatedAndStopWordsKeepBaseline(t *testing.T) {
	for _, tc := range []struct {
		a, b  string
		match bool
		score float64
	}{
		{"Strategist Strategist", "Global Principal Retail Brand Strategist", false, 0},
		{"Global Principal Retail Brand Strategist", "Strategist Strategist", false, 0},
		{"Brand Strategist Strategist", "Strategist Global Principal Retail Brand", false, 0},
		{"Strategist Global Principal Retail Brand", "Brand Strategist Strategist", false, 0},
		{"Manager", "Deputy Manager of Strategist Strat Senior", false, 0},
		{"Sr of Brand", "Brand Sr VP Chief Strategist", false, 0},
		{"Principal the", "Chief Strategist Deputy Sr the Principal", false, 0},
	} {
		got, score := FuzzyScore("Title", tc.a, tc.b, false)
		if got != tc.match || score != tc.score {
			t.Errorf("%q / %q: got %v %v; baseline %v %v", tc.a, tc.b, got, score, tc.match, tc.score)
		}
	}
}
