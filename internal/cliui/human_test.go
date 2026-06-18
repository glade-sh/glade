package cliui

import "testing"
import "bytes"

func TestOutputBudgetCapsRowsAndCountsOmitted(t *testing.T) {
	budget := OutputBudget{}
	if got := budget.EffectiveLimit(); got != 80 {
		t.Fatalf("default limit = %d, want 80", got)
	}
	if got := budget.VisibleCount(95); got != 80 {
		t.Fatalf("visible count = %d, want 80", got)
	}
	if got := budget.OmittedCount(95); got != 15 {
		t.Fatalf("omitted count = %d, want 15", got)
	}
	if got := (OutputBudget{Limit: 3}).VisibleCount(2); got != 2 {
		t.Fatalf("visible count below limit = %d, want 2", got)
	}
}

func TestHumanSectionAndKeyValueHelpers(t *testing.T) {
	var out bytes.Buffer
	if err := WriteSection(&out, "Summary"); err != nil {
		t.Fatal(err)
	}
	if err := WriteKeyValue(&out, "Objects", 3); err != nil {
		t.Fatal(err)
	}
	got := out.String()
	if got != "\nSummary:\n  Objects  3\n" {
		t.Fatalf("output = %q", got)
	}
}
