package dml

import "testing"

// NATIVE company_name_fit/org.tsv:308,316 (D1_nd, D1_nd2), :300 (D1_jn),
// :324 (D1_qf), istest 62.0. Each row is one directed pair the shape gates
// decide.
func TestCompanyD1ShapeGates(t *testing.T) {
	for _, c := range []struct {
		candidate, stored string
		match             bool
	}{
		// Same words with different numbers match; a differing word with
		// unrelated numbers does not.
		{"Zorvex Region 1 Sales", "Zorvex Region 2 Sales", true},
		{"Zorvex Region 1", "Zorvex Region 2 Sales", false},
		{"Zorvex Region 1 West", "Zorvex Region 2 East", false},
		{"Zorvex Region 1", "Zorvex Region 1 Sales", true},
		{"Zorvex Region 1", "Zorvex Region 11 Sales", true},
		{"Zorvex Region 2", "Zorvex Region 11 Sales", false},
		{"Zorvex Region", "Zorvex Region 1 Sales", false},
		{"Zorvex Region1", "Zorvex Region 1 Sales", false},
		{"Zorvex Region1", "Zorvex Region2", true},
		// Joins: an underscore join does not match, a hyphen join needs alike
		// words, and a doubled letter matches.
		{"Zorvex_Marine Supply", "Zorvex Marine Supply", false},
		{"Quen-State Paving", "Quen State Painting", false},
		{"Vexlett-Packard", "Vexlet Packard", true},
		{"Vexlett Packard", "Vexlet Packard", true},
		// Qualifiers: a longer suffix or base does not match across forms;
		// a dash form finds a similar suffix only one way.
		{"Zorvex Freight/West", "Zorvex Freight (West Coast)", false},
		{"Zorvex Freight/West", "Zorvex Freight Services (West)", false},
		{"Zorvex Freight/West", "Zorvex Freight/East", true},
		{"Zorvex Freight/West", "Zorvex Freight/North", false},
		{"Zorvex Freights/West", "Zorvex Freight/West", true},
		{"Zorvex Freight/East", "Zorvex Freights/West", true},
		{"Zorvex Freights/West", "Zorvex Freight/East", false},
		{"Zorvex Freight (West)", "Zorvex Freight Services (West)", true},
		{"Zorvex Freight (East)", "Zorvex Freight Services (West)", false},
		{"Zorvex Freight / West", "Zorvex Freight West", true},
		{"Zorvex Freight/East", "Zorvex Freight - West", true},
		{"Zorvex Freight - West", "Zorvex Freight/East", false},
	} {
		if got, _ := fuzzyCompany(c.candidate, c.stored, false); got != c.match {
			t.Errorf("%q -> %q: got %v, want %v", c.candidate, c.stored, got, c.match)
		}
	}
}

// Doubled letters match only multiword names, and the standard rule keeps the
// word scorer.
func TestCompanyDoubledEqual(t *testing.T) {
	for _, c := range []struct {
		a, b string
		want bool
	}{
		{"Vexlett Packard", "Vexlet Packard", true},
		{"VEXLETT-PACKARD", "vexlet-packard", true},
		{"Vexlett", "Vexlet", false},
		{"Vexlet Packard", "Vexlet Packard", false},
		{"Vexlett Packard", "Vexlet Packerd", false},
		{"Vexlett Packard", "Vexlett Packard Inc", false},
		{"Café Packard", "Cafe Packard", false},
	} {
		if got := companyDoubledEqual(c.a, c.b); got != c.want {
			t.Errorf("%q / %q: got %v, want %v", c.a, c.b, got, c.want)
		}
	}
	got, score := standardCompany("Vexlett Packard", "Vexlet Packard")
	want, wantScore := false, 0.0
	if standardCompanyGate("Vexlett Packard", "Vexlet Packard") {
		want, wantScore = legacyFuzzyCompany("Vexlett Packard", "Vexlet Packard", false)
	}
	if got != want || score != wantScore {
		t.Errorf("standard rule: got %v %v, want %v %v", got, score, want, wantScore)
	}
}
