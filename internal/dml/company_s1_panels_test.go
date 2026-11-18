package dml

import (
	"math"
	"testing"
)

// NATIVE company_name_fit/org.tsv s1_a to s1_d (custom CompanyName rule, API
// 62.0 and 67.0, anonymous and @IsTest, two repeats; all observations agree).
// s1_a and s1_c hold a parenthesis candidate "(East)" against a stored " -
// West" on 26 bases; s1_b holds E, O and U suffix words on six bases; s1_d
// holds word endings on one stem. Every ordered pair is compared. Glade makes
// no false match on any panel. The misses are pinned in companyS1Pinned.
func TestCompanyS1PanelCounts(t *testing.T) {
	want := map[string][4]int{
		"s1_a": {33, 26, 26, 0},
		"s1_b": {66, 60, 60, 0},
		"s1_c": {32, 24, 24, 0},
		"s1_d": {87, 71, 70, 0},
	}
	panels := companyFitPanels(t, "s1_")
	if len(panels) != len(want) {
		t.Fatalf("%d panels, want %d", len(panels), len(want))
	}
	for name, panel := range panels {
		hits, exact, extra := 0, 0, 0
		for i := range panel.values {
			for j := range panel.values {
				got, score := fuzzyCompany(panel.values[i], panel.values[j], false)
				native, matched := panel.native[[2]int{i, j}]
				switch {
				case got && matched:
					hits++
					if math.Round(score) == native {
						exact++
					}
				case got:
					extra++
				}
			}
		}
		w := want[name]
		t.Logf("%s: native %d, matched %d, exact confidence %d, not native %d", name, len(panel.native), hits, exact, extra)
		if len(panel.native) != w[0] || hits < w[1] || exact < w[2] || extra > w[3] {
			t.Errorf("%s: native %d matched %d exact %d not native %d, want %d, at least %d, at least %d, at most %d", name, len(panel.native), hits, exact, extra, w[0], w[1], w[2], w[3])
		}
	}
}

// Two parenthesis forms on one base that differ only in the E, O or U suffix
// word match at 100 in both directions on every captured base.
// companyVowelParenScore fires for any vowel pair; I and A are untested.
func TestCompanyVowelParenConfidence(t *testing.T) {
	bases := []string{"Orbel Trade", "Zorvex Fret", "Brix Marine", "Mandolar Bank", "Plovdi Trade", "Zorvex Audio"}
	suffixes := []string{"East", "Oast", "Uast"}
	for _, base := range bases {
		for _, x := range suffixes {
			for _, y := range suffixes {
				if x == y {
					continue
				}
				a, b := base+" ("+x+")", base+" ("+y+")"
				if got, score := fuzzyCompany(a, b, false); !got || math.Round(score) != 100 {
					t.Errorf("%q vs %q: %v %.0f, want match at 100", a, b, got, score)
				}
			}
		}
	}
}

// companyS1Pinned are native matches Glade does not report; each row is a
// miss kept as captured until a rule that holds across bases is fitted.
// s1_a and s1_c: a parenthesis candidate "(East)" matches a stored " - West"
// at 90 to 93 on some bases only, with no separator in length, word class
// or vowel identity. s1_d: the "Vexlet" stem has no second base for the
// endings that miss.
var companyS1Pinned = []struct {
	panel, candidate, stored string
	native                   float64
}{
	{"s1_a", "Ormadi Trade (East)", "Ormadi Trade - West", 91},
	{"s1_a", "Brixen Marine (East)", "Brixen Marine - West", 91},
	{"s1_a", "Brixen Bank (East)", "Brixen Bank - West", 90},
	{"s1_a", "Kelmor Freightways (East)", "Kelmor Freightways - West", 91},
	{"s1_a", "Plenvaro Trade (East)", "Plenvaro Trade - West", 92},
	{"s1_a", "Plenvaro Audio (East)", "Plenvaro Audio - West", 90},
	{"s1_a", "Qelvantor Trade (East)", "Qelvantor Trade - West", 92},
	{"s1_b", "Zorvex Fret (East)", "Zorvex Fret - West", 90},
	{"s1_b", "Brix Marine (East)", "Brix Marine - West", 91},
	{"s1_b", "Brix Marine (Oast)", "Brix Marine - West", 90},
	{"s1_b", "Brix Marine (Uast)", "Brix Marine - West", 90},
	{"s1_b", "Mandolar Bank (East)", "Mandolar Bank - West", 90},
	{"s1_b", "Plovdi Trade (East)", "Plovdi Trade - West", 91},
	{"s1_c", "Zorvex Fret (East)", "Zorvex Fret - West", 90},
	{"s1_c", "Orbel Tradex (East)", "Orbel Tradex - West", 91},
	{"s1_c", "Orbel Trading (East)", "Orbel Trading - West", 91},
	{"s1_c", "Plovdi Trade (East)", "Plovdi Trade - West", 91},
	{"s1_c", "Brix Marine (East)", "Brix Marine - West", 91},
	{"s1_c", "Strng Trading (East)", "Strng Trading - West", 93},
	{"s1_c", "Strng Trading - West", "Strng Trading (East)", 90},
	{"s1_c", "Bandrel Trade (East)", "Bandrel Trade - West", 91},
	{"s1_d", "Vexlets Packard", "Vexletronics Packard", 91},
	{"s1_d", "Vexlets Packard", "Vexlettt Packard", 91},
	{"s1_d", "Qelthorins Freight", "Qelthoriner Freight", 100},
	{"s1_d", "Qelthoriner Freight", "Qelthorins Freight", 100},
	{"s1_d", "Vexleta Packard", "Vexletob Packard", 97},
	{"s1_d", "Vexletac Packard", "Vexletob Packard", 96},
	{"s1_d", "Vexletob Packard", "Vexleta Packard", 96},
	{"s1_d", "Vexletob Packard", "Vexletac Packard", 96},
	{"s1_d", "Vexlettt Packard", "Vexlets Packard", 97},
	{"s1_d", "Vexlettt Packard", "Vexleta Packard", 94},
	{"s1_d", "Vexlettt Packard", "Vexletab Packard", 93},
	{"s1_d", "Vexlettt Packard", "Vexletac Packard", 93},
	{"s1_d", "Vexlettt Packard", "Vexletob Packard", 93},
	{"s1_d", "Vexlett Packard", "Vexletab Packard", 93},
	{"s1_d", "Vexlett Packard", "Vexletac Packard", 93},
	{"s1_d", "Vexlett Packard", "Vexletob Packard", 93},
}

func TestCompanyS1PinnedMisses(t *testing.T) {
	for _, row := range companyS1Pinned {
		if got, score := fuzzyCompany(row.candidate, row.stored, false); got {
			t.Errorf("%s: %q vs %q now matches at %.0f (native %.0f): fit the rule and unpin the row", row.panel, row.candidate, row.stored, score, row.native)
		}
	}
	// NATIVE s1_d: "Vexlet Inc" finds "Vexlets Inc" at 93; Glade reports 91.
	if got, score := fuzzyCompany("Vexlet Inc", "Vexlets Inc", false); !got || math.Round(score) != 91 {
		t.Errorf("Vexlet Inc vs Vexlets Inc: %v %.0f, want the pinned 91 (native 93)", got, score)
	}
}
