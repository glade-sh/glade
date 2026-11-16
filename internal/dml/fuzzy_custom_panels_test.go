package dml

import (
	"math"
	"testing"
)

// The custom-rule extensions apply only to the shapes the duplicate_fuzzy_panels
// rows cover. These neighbouring shapes, none of them captured, keep the
// verdict and score origin/main gives them: keyboard edits in names under ten
// letters, over thirteen, or in the first five letters; initials against
// names, two longer initials and initials outside a to z; first words that
// differ inside the match key or with a different second word; plurals outside
// the captured lengths or already matched by a proven word pair; titles of seven
// words. Street has its own scorer and test (TestCustomStreetNeighbourShapes).
// The keyboard fallback scores "Brandenstock"/"Brandewxtovz" (shape (12, 4, 0),
// 91.18, above the shape's floor) as it scores the native match
// "Brandensjojn"/"Brandepswohk" of the same shape and score (NATIVE
// duplicate_fuzzy_panels/org.tsv:34 k1 pair 12, 96).
func TestCustomFuzzyExtensionsKeepNeighbourShapes(t *testing.T) {
	for _, c := range []struct {
		method, a, b string
		match        bool
		score        float64
	}{
		{"LastName", "Pemberly", "Pemvwrly", false, 0.0000},
		{"LastName", "Brandensto", "Brandenspo", true, 98.4000},
		{"LastName", "Brandenstockers", "Brandenspockerz", true, 98.7667},
		{"LastName", "Brandenstock", "Brandenstoxj", true, 100.0000},
		{"LastName", "Brandenstock", "Brqndenstock", false, 0.0000},
		{"LastName", "Kellingsworth", "Kellingswortz", true, 98.7692},
		{"LastName", "Tolvermandel", "Tolverwanqez", false, 95.3333},
		{"LastName", "Brandenstock", "Brandewxtovz", true, 95.5917},
		{"Title", "Global Principal Retail Brand Strategist", "Strategist", false, 0.0000},
		{"Title", "Brand Strategist", "Deputy Senior Global Principal Retail Brand Strategist", true, 91.0000},
		{"Title", "Strategist", "Deputy Senior Global Principal Retail Brand Strategist", false, 0.0000},
		{"Title", "Retail Brand Strategist", "Deputy Senior Global Principal Retail Brand Strategist", true, 94.0000},
		{"Title", "Brand Strategist", "Strategist Deputy Senior Global Principal Retail Brand", false, 0.0000},
		{"Title", "Brand Strategist Lead", "Strategist Global Principal Retail Brand", false, 0.0000},
		{"Title", "Senior Strategist", "Senior Retail Strategist", true, 95.0000},
	} {
		match, score := FuzzyScore(c.method, c.a, c.b, false)
		if match != c.match || math.Abs(score-c.score) > 1e-3 {
			t.Errorf("%s %q/%q: got %v %.4f, origin/main %v %.4f", c.method, c.a, c.b, match, score, c.match, c.score)
		}
	}
}

// The custom Street scorer on the neighbours of the duplicate_fuzzy_panels
// street rows. A street without a number, or with another number, stays apart;
// an unknown word one edit from the suffix matches at edit similarity 87 and
// above. NATIVE fuzzy_street_c5/org.tsv nd4: a trailing directional does not
// change that ("84 Sorrel St N" finds "84 Sorrel Strt N").
func TestCustomStreetNeighbourShapes(t *testing.T) {
	for _, c := range []struct {
		a, b  string
		match bool
		conf  float64
	}{
		{"418 Larkwhistle St", "418 Larkwhistle Sreet", false, 0}, // 85.71
		{"418 Larkwhistle St N", "418 Larkwhistle Strt N", true, 82},
		{"Larkwhistle St", "Larkwhistle Strt", false, 0},
		{"418 Larkwhistle St", "419 Larkwhistle Strt", false, 0},
		{"418 Larkwhistle Rd", "418 Larkwhistle Rdx", true, 88},
		{"418 Larkwhistle St", "418 Larkwhistle Ste", true, 88},
	} {
		match, score := FuzzyScore("Street", c.a, c.b, false)
		if match != c.match || match && math.Floor(score+0.5) != c.conf {
			t.Errorf("Street %q/%q: got %v %.2f, want %v %.0f", c.a, c.b, match, score, c.match, c.conf)
		}
	}
}

// NATIVE duplicate_fuzzy_panels/org.tsv: one captured pair per extension,
// checked through the custom scorer and kept away from the shared scorer the
// standard rules use.
func TestCustomFuzzyExtensionsCapturedPairs(t *testing.T) {
	for _, c := range []struct {
		method, a, b string
		score        float64
	}{
		{"Street", "418 Larkwhistle St", "418 Larkwhistle Strt", 84},
		{"Street", "72 Dunholt Drive", "72 Dunholt Drv", 81},
		{"Title", "Strategist", "Global Principal Retail Brand Strategist", 88},
		{"Title", "Brand Strategist", "Strategist Global Principal Retail Brand", 90},
		{"Title", "Global Principal Retail Brand Strategist", "Deputy Global Principal Retail Brand Strategist", 98},
	} {
		match, score := FuzzyScore(c.method, c.a, c.b, false)
		if !match || math.Floor(score+0.5) != c.score {
			t.Errorf("%s %q/%q: got %v %.2f, native %.0f", c.method, c.a, c.b, match, score, c.score)
		}
	}
	for _, c := range []struct{ method, a, b string }{
		{"Street", "418 Larkwhistle St", "418 Larkwhistle Sreet"},
		{"Title", "Global Principal Retail Brand Strategist", "Strategist"},
	} {
		if match, _ := FuzzyScore(c.method, c.a, c.b, false); match {
			t.Errorf("%s %q/%q matched; native keeps them apart", c.method, c.a, c.b)
		}
	}
	// The standard rules call the shared scorers, which these rows do not
	// change. Without a match key the Last Name steps stop at the base
	// Keyboard Distance (lastNameAlgorithms).
	if match, _ := lastNameAlgorithms("brandenstock", "brandepswohk"); match {
		t.Error("shared Last Name scorer changed")
	}
	if match, _ := fuzzyStreet("418 Larkwhistle St", "418 Larkwhistle Strt", false); match {
		t.Error("shared Street scorer changed")
	}
}
