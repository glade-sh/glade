package cliui

import "testing"

func TestColorEnabled(t *testing.T) {
	oldGetTerm := getTerm
	getTerm = func() string { return "" }
	t.Cleanup(func() { getTerm = oldGetTerm })
	if ColorEnabled(false, "") {
		t.Fatal("non-TTY output must not use color")
	}
	if ColorEnabled(true, "1") {
		t.Fatal("NO_COLOR must disable color")
	}
	if !ColorEnabled(true, "") {
		t.Fatal("TTY output without NO_COLOR should allow color")
	}
	getTerm = func() string { return "dumb" }
	if ColorEnabled(true, "") {
		t.Fatal("TERM=dumb must disable color")
	}
}
