package dml

import (
	"math"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE company_name_fit/org.tsv R3_* (custom CompanyName rule, API 62.0 and
// 67.0, anonymous and @IsTest, two repeats; all observations agree). The
// panels isolate first words that differ past the six-letter match key,
// vowel suffixes across dash, parenthesis and plain forms on bases of
// different lengths, and glued digit runs. Every ordered pair is compared;
// the counts are as in TestCompanyShapePanelCounts.
func TestCompanyIsolationPanelCounts(t *testing.T) {
	want := map[string][4]int{
		"R3_key":  {130, 112, 111, 0},
		"R3_base": {74, 74, 42, 0},
		"R3_long": {123, 105, 38, 0},
		"R3_glu":  {131, 131, 112, 0},
	}
	panels := companyFitPanels(t, "R3_")
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

// companyIsolationRows are directed pairs, candidate first, from the R1, R2
// and R3 panels. A confidence of 0 checks only the decision.
var companyIsolationRows = []struct {
	panel, candidate, stored string
	match                    bool
	confidence               float64
}{
	// First words that share the match key: one extends the other, or both
	// have one length and differ in one letter after the key.
	{"R1_dbl", "Vexlett Packard", "Vexlets Packard", true, 100},
	{"R1_dbl", "Vexlets Packard", "Vexlet Packard", true, 100},
	{"R1_dbl", "Vexlet Packardd", "Vexlett Packard", true, 100},
	{"R1_dbl", "Vexlet Packard Inc", "Vexlettt Packard", true, 100},
	{"R1_dbl", "Vexlett Packard Inc", "Vexlet Packard", true, 100},
	{"R3_key", "Vexletronics Packard", "Vexlet Packard", true, 100},
	{"R3_key", "Vexlet Packard", "Vexletcd Packard", true, 100},
	{"R3_key", "Vexleta Packard", "Vexletab Packard", true, 100},
	{"R3_key", "Vexlets Packard", "Vexleta Packard", true, 100},
	{"R3_key", "Qelthorin Freight", "Qelthoriner Freight", true, 100},
	{"R3_key", "Zorvexs Marine Supply", "Zorvexa Marine Supply", true, 100},
	{"R3_key", "Vexlet Packard Group", "Vexleta Packard", true, 91},
	{"R3_key", "Vexlets Packard", "Vexleter Packard", false, 0},
	{"R3_key", "Vexletab Packard", "Vexletcd Packard", false, 0},
	{"R3_key", "Vexle Packard", "Vexles Packard", false, 0},
	{"R3_key", "Vexlets", "Vexlet", false, 0},
	{"R3_key", "Vexleta Packard", "Vexlettt Packard", false, 0},
	{"R3_key", "Vexlets Packard Group", "Vexletab Packard", false, 0},
	{"R1_dbl", "Vexllet Packard", "Vexlett Packard", false, 0},
	{"R1_dbl", "Vexlet Packkard", "Vexlet Packard", false, 0},
	// Two spaced slash forms score as two dash forms; a spaced slash against a
	// plain or parenthesis form scores as the dash form.
	{"R2_vow", "Orbel Trade / West", "Orbel Trade / East", true, 0},
	{"R2_vow", "Orbel Trade / East", "Orbel Trade / West", true, 0},
	{"R2_vow", "Orbel Trade / East", "Orbel Trade / Oast", true, 0},
	{"R2_par", "Zorvex Freight / Oast", "Zorvex Freight / Best", true, 0},
	{"R2_vow", "Orbel Trade / East", "Orbel Trade West", true, 91},
	{"R2_vow", "Orbel Trade East", "Orbel Trade / Mast", true, 94},
	{"R2_par", "Zorvex Freight / East", "Zorvex Freight (West)", true, 0},
	{"R2_vow", "Orbel Trade / Mast", "Orbel Trade West", false, 0},
	// A dash candidate with a vowel suffix against a plain name.
	{"R2_vow", "Orbel Trade - East", "Orbel Trade West", true, 91},
	{"R2_vow", "Orbel Trade - East", "Orbel Trade Mast", true, 91},
	{"R2_vow", "Orbel Trade - Oast", "Orbel Trade East", true, 94},
	{"R3_long", "Qa Trade - East", "Qa Trade West", true, 0},
	{"R2_vow", "Orbel Trade - Oast", "Orbel Trade West", false, 0},
	{"R2_vow", "Orbel Trade - Ease", "Orbel Trade Mast", false, 0},
	{"R2_vow", "Orbel Trade - West", "Orbel Trade East", false, 0},
	// A plain candidate with a vowel suffix against a dash name.
	{"R2_vow", "Orbel Trade East", "Orbel Trade - West", true, 93},
	{"R2_par", "Zorvex Freight East", "Zorvex Freight - Mast", true, 94},
	{"R2_vow", "Orbel Trade Oast", "Orbel Trade - West", true, 0},
	{"R2_vow", "Orbel Trade East", "Orbel Trade - Oast", true, 96},
	{"R3_long", "Qa Trade East", "Qa Trade - West", true, 0},
	{"R2_vow", "Orbel Trade East", "Orbel Trade - Ease", false, 0},
	{"R2_vow", "Orbel Trade West", "Orbel Trade - East", false, 0},
	// A vowel suffix across a dash and a parenthesis form: a first-letter
	// change matches on every base, other consonant suffixes by the length
	// of the whole name.
	{"R3_base", "Orbel Trade (East)", "Orbel Trade - Wast", true, 91},
	{"R3_base", "Orbel Trade (East)", "Orbel Trade - West", false, 0},
	{"R3_base", "Orbel Trading (East)", "Orbel Trading - West", true, 0},
	{"R3_base", "Orbel Tradex - East", "Orbel Tradex (West)", true, 0},
	{"R3_base", "Zorvex Fret - East", "Zorvex Fret (West)", true, 0},
	{"R3_base", "Orbelvanto Trade (East)", "Orbelvanto Trade - West", true, 0},
	{"R3_base", "Qa Freight (East)", "Qa Freight - West", false, 0},
	{"R3_base", "Qa Freight - East", "Qa Freight (West)", false, 0},
	{"R3_long", "Qa Trade (East)", "Qa Trade - West", false, 0},
	{"R3_long", "Orbel Trade (Oast)", "Orbel Trade - Mast", true, 0},
	{"R2_par", "Zorvex Freight (Oast)", "Zorvex Freight - Mast", true, 0},
	{"R2_par", "Zorvex Freight (Oast)", "Zorvex Freight - Best", false, 0},
	{"R2_par", "Zorvex Freight - Mast", "Zorvex Freight (Oast)", false, 0},
	{"R2_par", "Zorvex Freight (East)", "Zorvex Freight - Oast", true, 0},
	{"R1_dir", "Orbel Trade - East", "Orbel Trade (West)", false, 0},
	// Glued digit runs on one multiword base.
	{"R2_glu", "Test Account1", "Test Account21", true, 100},
	{"R2_glu", "Test Account13", "Test Account123", true, 100},
	{"R2_glu", "Test Account1", "Test Account123", true, 100},
	{"R2_glu", "Test Account3", "Test Account123", true, 97},
	{"R2_glu", "Test Account1", "Test Account20", true, 95},
	{"R2_glu", "Test Account456", "Test Account1", true, 91},
	{"R2_glu", "Test Account3", "Test Account133", true, 98},
	{"R2_glu", "Test Account10", "Test Account456", true, 92},
	{"R2_glu", "Qelvun Sky Farms1", "Qelvun Sky Farms22", true, 98},
	{"R1_dig", "Test Account1", "Test Account01", true, 100},
	{"R3_glu", "Zorvex Sky Port56", "Zorvex Sky Port65", true, 97},
	{"R3_glu", "Zorvex Sky Port6", "Zorvex Sky Port556", true, 99},
	{"R3_glu", "Orbel Trade7", "Orbel Trade8", false, 0},
}

// Each row is checked against the captured panel and against Glade.
func TestCompanyIsolationRows(t *testing.T) {
	panels := companyFitPanels(t, "R1_", "R2_", "R3_")
	for _, c := range companyIsolationRows {
		panel := panels[c.panel]
		i, j := -1, -1
		for k, v := range panel.values {
			if v == c.candidate {
				i = k
			}
			if v == c.stored {
				j = k
			}
		}
		if i < 0 || j < 0 {
			t.Fatalf("%s: %q or %q is not in the panel", c.panel, c.candidate, c.stored)
		}
		native, matched := panel.native[[2]int{i, j}]
		if matched != c.match || c.confidence != 0 && native != c.confidence {
			t.Fatalf("%s %q -> %q: row says %v %v, capture says %v %v", c.panel, c.candidate, c.stored, c.match, c.confidence, matched, native)
		}
		got, score := fuzzyCompany(c.candidate, c.stored, false)
		if got != c.match || c.confidence != 0 && math.Round(score) != c.confidence {
			t.Errorf("%s %q -> %q: got %v %v, want %v %v", c.panel, c.candidate, c.stored, got, score, c.match, c.confidence)
		}
		blankGot, blankScore := fuzzyCompany(c.candidate, c.stored, true)
		if blankGot != got || blankScore != score {
			t.Errorf("%s %q -> %q: MatchBlanks got %v %v, NullNotAllowed %v %v", c.panel, c.candidate, c.stored, blankGot, blankScore, got, score)
		}
	}
}

// The isolation rows through the shared duplicate-rule matcher and through a
// blocking insert: a captured match is found and blocks the save at its
// confidence, a captured non-match saves.
func TestCompanyIsolationRowsRoute(t *testing.T) {
	for _, c := range companyIsolationRows {
		org := companyPairOrg("N3Co", c.stored)
		result, _ := FindDuplicateMatches(org, companyPairRule("N3Co", "Allow"), companyPairRecord(c.candidate), nil)
		found, confidence := len(result.Matches) == 1 && len(result.Matches[0].Records) == 1, 0.0
		if found {
			confidence = math.Floor(result.Matches[0].Confidences[0] + 0.5)
		}
		if found != c.match || found && c.confidence != 0 && confidence != c.confidence {
			t.Errorf("find %q -> %q: got %v %v, want %v %v", c.candidate, c.stored, found, confidence, c.match, c.confidence)
		}
		state := org.Objects["Account"]
		state.Definition.DuplicateRules = []storage.DuplicateRule{companyPairRule("N3Co", "Block")}
		org.Objects["Account"] = state
		engine := NewEngine(org)
		engine.Options.EnforceDuplicateRules = true
		results := engine.Insert([]storage.Record{companyPairRecord(c.candidate)})
		if len(results) != 1 {
			t.Fatalf("%d results", len(results))
		}
		if blocked := !results[0].Success; blocked != c.match {
			t.Errorf("insert %q over %q: outcome %s, want blocked %v", c.candidate, c.stored, companyPairOutcome(results[0]), c.match)
		}
	}
}
