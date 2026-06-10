package cliui

import (
	"strings"
	"testing"
)

func TestFormatBoxPlain(t *testing.T) {
	th := NewPlainTheme()
	got := FormatBox(th, "Tests", "12 selected · 11 passed", 40)
	if !strings.Contains(got, "Tests") {
		t.Fatalf("missing title: %q", got)
	}
	if !strings.Contains(got, "12 selected") {
		t.Fatalf("missing body: %q", got)
	}
	if !strings.HasPrefix(got, "+") {
		t.Fatalf("expected ascii box: %q", got)
	}
	for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
		if w := VisibleWidth(line); w != 40 {
			t.Fatalf("line width = %d, want 40: %q", w, line)
		}
	}
}

func TestFormatBoxUnicodeRowsAlign(t *testing.T) {
	th := Theme{Color: true, PlainBox: false}
	got := FormatBox(th, "Tests", "6 selected · 6 passed · 0 failed · 3s", 80)
	for _, line := range strings.Split(strings.TrimSuffix(got, "\n"), "\n") {
		if w := VisibleWidth(line); w != 80 {
			t.Fatalf("line width = %d, want 80: %q", w, line)
		}
	}
}

func TestVisibleWidthIgnoresANSI(t *testing.T) {
	s := "\x1b[32mok\x1b[0m"
	if VisibleWidth(s) != 2 {
		t.Fatalf("width = %d, want 2", VisibleWidth(s))
	}
}

func TestFormatDurationMS(t *testing.T) {
	if got := FormatDurationMS(36); got != "36ms" {
		t.Fatalf("got %q", got)
	}
	if got := FormatDurationMS(1500); got != "2s" {
		t.Fatalf("got %q", got)
	}
}
