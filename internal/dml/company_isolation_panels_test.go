package dml

import (
	"math"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// NATIVE company_name_fit/org.tsv R3_* to R7_* (custom CompanyName rule, API
// 62.0 and 67.0, anonymous and @IsTest, two repeats; all observations agree).
// R3 isolates first words that differ past the six-letter match key, vowel
// suffixes across dash, parenthesis and plain forms on bases of different
// lengths, and glued digit runs. R4 to R7 test those rules on names the rules
// were not fitted on: R4_ew, R4_ob, R5_*, R6_* and R7_flo hold one vowel and
// one consonant suffix across a dash and a parenthesis form on 100 bases,
// R7_pl and R7_sp the dash, plain and spaced slash forms on other bases,
// R4_misc other suffix lengths, R4_sp spaced slash suffixes of different
// lengths, R4_stem first words before a legal designator and R4_glu digit
// runs on other bases. Every ordered pair is compared; the counts are as in
// TestCompanyShapePanelCounts. A vowel suffix against a consonant suffix
// across a dash and a parenthesis form, and a dash candidate with a vowel
// suffix against a plain name, match natively on some bases only; Glade
// reports none of them, so those panels count no match beyond the names
// themselves.
func TestCompanyIsolationPanelCounts(t *testing.T) {
	want := map[string][4]int{
		"R3_key":  {130, 112, 111, 0},
		"R3_base": {74, 65, 37, 0},
		"R3_long": {123, 92, 37, 0},
		"R3_glu":  {131, 131, 112, 0},
		"R4_ew":   {33, 26, 26, 0},
		"R4_ob":   {50, 42, 34, 0},
		"R4_misc": {257, 230, 127, 6},
		"R4_sp":   {228, 197, 69, 2},
		"R4_stem": {83, 81, 69, 0},
		"R4_glu":  {280, 278, 267, 20},
		"R5_hi1":  {33, 26, 26, 0},
		"R5_hi2":  {38, 26, 26, 0},
		"R5_hi3":  {42, 26, 26, 0},
		"R6_ew1":  {37, 26, 26, 0},
		"R6_ew2":  {38, 26, 26, 0},
		"R6_mix":  {49, 26, 26, 0},
		"R7_flo":  {30, 26, 26, 0},
		"R7_pl":   {92, 84, 37, 1},
		"R7_sp":   {108, 104, 53, 4},
	}
	panels := companyFitPanels(t, "R3_", "R4_", "R5_", "R6_", "R7_")
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

// companyIsolationRows are directed pairs, candidate first, from the R1 to R7
// panels. A confidence of 0 checks only the decision.
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
	{"R4_stem", "Vexlets Group", "Vexlet Group", true, 100},
	{"R4_stem", "Vexlets Holdings", "Vexlet Holdings", true, 100},
	{"R4_stem", "Vexlets P", "Vexlet P", true, 100},
	{"R4_stem", "Consult Bank", "Consultants Bank", true, 100},
	{"R4_stem", "Vexletab Packard", "Vexletac Packard", true, 100},
	{"R4_stem", "Mandolared Packard", "Mandolarer Packard", true, 100},
	{"R4_stem", "Vexlets and Packard", "Vexlet Packard", true, 100},
	// A first word before a legal designator alone is a one-word name.
	{"R4_stem", "Vexlets Inc", "Vexlet Inc", false, 0},
	{"R4_stem", "Vexlets Corp", "Vexlet LLC", false, 0},
	{"R4_stem", "Vexletronics Inc", "Vexlet Company", false, 0},
	{"R4_stem", "Vexleta Inc", "Vexlets Co", false, 0},
	// Two spaced slash forms score as two dash forms, whatever the suffix
	// lengths; a plain vowel candidate finds a spaced slash as it finds the
	// dash form.
	{"R2_vow", "Orbel Trade / West", "Orbel Trade / East", true, 0},
	{"R2_vow", "Orbel Trade / East", "Orbel Trade / West", true, 0},
	{"R2_vow", "Orbel Trade / East", "Orbel Trade / Oast", true, 0},
	{"R2_par", "Zorvex Freight / Oast", "Zorvex Freight / Best", true, 0},
	{"R2_vow", "Orbel Trade East", "Orbel Trade / Mast", true, 94},
	{"R2_par", "Zorvex Freight / East", "Zorvex Freight (West)", true, 0},
	{"R2_vow", "Orbel Trade / Mast", "Orbel Trade West", false, 0},
	{"R4_sp", "Orbel Trade / Art", "Orbel Trade / North", true, 0},
	{"R4_sp", "Orbel Trade / Ok", "Orbel Trade / Outer", true, 0},
	{"R4_sp", "Orbel Trade / North", "Orbel Trade / West", true, 0},
	{"R7_sp", "Mandolar Oil / East", "Mandolar Oil / North", true, 0},
	{"R7_sp", "Zorvex Audio / West", "Zorvex Audio / East", true, 0},
	{"R7_sp", "Vexlet Bank East", "Vexlet Bank / West", true, 93},
	{"R7_sp", "Mandolar Oil / East", "Mandolar Oil West", false, 0},
	// A dash candidate with a vowel suffix against a plain name matches on
	// "Orbel Trade", "Zorvex Freight" and "Qa Trade" (91 to 94) and not on
	// the bases below; Glade reports none.
	{"R7_pl", "Mandolar Oil - East", "Mandolar Oil West", false, 0},
	{"R7_pl", "Ab Logistics - East", "Ab Logistics West", false, 0},
	{"R7_pl", "Mandolar Oil - Oast", "Mandolar Oil Mast", false, 0},
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
	{"R7_pl", "Mandolar Oil East", "Mandolar Oil - West", true, 0},
	{"R7_pl", "Kio Freightways East", "Kio Freightways - West", true, 0},
	{"R7_pl", "Orbel Oil Outlet East", "Orbel Oil Outlet - West", true, 0},
	{"R7_pl", "Ab Logistics West", "Ab Logistics - East", false, 0},
	{"R4_misc", "Zorvex Freight Art", "Zorvex Freight - Bat", false, 0},
	// A vowel suffix across a dash and a parenthesis form matches on some
	// bases ("Zorvex Fret", "Orbel Trading", "Plovdi Trade") and not on
	// others, a first-letter change included; no score of the whole names
	// orders them. Glade reports none.
	{"R4_ew", "Orbel Freight (East)", "Orbel Freight - West", false, 0},
	{"R4_ew", "Zorvex Audio - East", "Zorvex Audio (West)", false, 0},
	{"R5_hi1", "Vexlet Bank (East)", "Vexlet Bank - West", false, 0},
	{"R5_hi2", "Kestrelton Oil - East", "Kestrelton Oil (West)", false, 0},
	{"R6_ew2", "Mandolar Outlet - East", "Mandolar Outlet (West)", false, 0},
	{"R6_mix", "Mandolar Oil Outlet (Oast)", "Mandolar Oil Outlet - Best", false, 0},
	{"R4_ob", "Mandolar Freight (Oast)", "Mandolar Freight - Best", false, 0},
	{"R4_ob", "Ab Co - East", "Ab Co (Wast)", false, 0},
	{"R7_flo", "Mandolar Oil (East)", "Mandolar Oil - Wast", false, 0},
	{"R7_flo", "Vexlet Bank - East", "Vexlet Bank (Wast)", false, 0},
	{"R7_flo", "Qa Freight - Oast", "Qa Freight (Mast)", false, 0},
	{"R4_misc", "Zorvex Freight (E)", "Zorvex Freight - W", false, 0},
	{"R3_base", "Orbel Trade (East)", "Orbel Trade - West", false, 0},
	{"R3_base", "Qa Freight (East)", "Qa Freight - West", false, 0},
	{"R3_base", "Qa Freight - East", "Qa Freight (West)", false, 0},
	{"R3_long", "Qa Trade (East)", "Qa Trade - West", false, 0},
	{"R2_par", "Zorvex Freight (Oast)", "Zorvex Freight - Best", false, 0},
	{"R2_par", "Zorvex Freight - Mast", "Zorvex Freight (Oast)", false, 0},
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
	{"R4_glu", "Test Account123456", "Test Account7", false, 0},
	{"R4_glu", "Ab Co456", "Ab Co1", false, 0},
	{"R4_glu", "Aeo Uia12346", "Aeo Uia1235", false, 0},
	// After a legal designator only a run that starts the other scores 100.
	{"R4_glu", "Test Account Inc1", "Test Account Inc12", true, 100},
	{"R4_glu", "Test Account Inc456", "Test Account Inc1", true, 93},
	{"R4_glu", "Test Account Inc2", "Test Account Inc12", true, 99},
	{"R4_glu", "Test Account Inc12", "Test Account Inc2", true, 98},
}

// Each row is checked against the captured panel and against Glade.
func TestCompanyIsolationRows(t *testing.T) {
	panels := companyFitPanels(t, "R1_", "R2_", "R3_", "R4_", "R5_", "R6_", "R7_")
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
