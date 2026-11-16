package dml

import "testing"

// The first-word stem rule needs two words of ASCII letters on both sides and
// leaves the Partner words to the word scorer.
func TestCompanyStemFirstWordShapes(t *testing.T) {
	for _, c := range []struct {
		a, b, want string
		ok         bool
	}{
		{"Vexlets Packard", "Vexlet Packard", "Vexlet Packard", true},
		{"VEXLETS Packard", "vexlet packard", "vexlet Packard", true},
		{" Vexlets  Packard", "Vexleta Packard", " Vexleta  Packard", true},
		{"Vexlets", "Vexlet", "", false},
		{"Vexlets Packard 2", "Vexlet Packard", "", false},
		{"Vexlets-Packard", "Vexlet Packard", "", false},
		{"Vexlets Packard", "Vexlets Packard", "", false},
		{"Partners Group", "Partner Group", "", false},
		{"Vexlé Packard", "Vexlet Packard", "", false},
		{"Vexlets Packard", "", "", false},
	} {
		got, ok := companyStemFirstWord(c.a, c.b)
		if got != c.want || ok != c.ok {
			t.Errorf("%q / %q: got %q %v, want %q %v", c.a, c.b, got, ok, c.want, c.ok)
		}
	}
	for _, c := range []struct {
		x, y string
		want bool
	}{
		{"1", "123", true}, {"1", "21", true}, {"13", "123", true}, {"123", "13", true},
		{"1", "01", true}, {"3", "123", false}, {"12", "21", false}, {"10", "21", false},
	} {
		if got := companyDigitsNear(c.x, c.y); got != c.want {
			t.Errorf("companyDigitsNear(%q, %q) = %v, want %v", c.x, c.y, got, c.want)
		}
	}
	for _, c := range []struct {
		a, b string
		ok   bool
	}{
		{"Orbel Trade / East", "Orbel Trade West", true},
		{"Orbel Trade / East", "Orbel Trade (West)", true},
		{"Orbel Trade / East", "Orbel Trade / West", true},
		{"Orbel Trade / East", "Orbel Trade - West", false},
		{"Orbel Trade / East", "Orbel Trade/West", false},
		{"Orbel Trade / West", "Orbel Trade West", false},
		{"Orbel Trade / East", "Orbel Trades West", false},
		{"Orbel Trade / West End", "Orbel Trade East", false},
		{"Orbel / East", "Orbel West", false},
	} {
		if _, _, ok := companySpacedAsDash(c.a, c.b); ok != c.ok {
			t.Errorf("companySpacedAsDash(%q, %q) = %v, want %v", c.a, c.b, ok, c.ok)
		}
	}
}
