package cliui

import "testing"

func TestColorEnabled(t *testing.T) {
	if ColorEnabled(false, "") {
		t.Fatal("non-TTY output must not use color")
	}
	if ColorEnabled(true, "1") {
		t.Fatal("NO_COLOR must disable color")
	}
	if !ColorEnabled(true, "") {
		t.Fatal("TTY output without NO_COLOR should allow color")
	}
}
