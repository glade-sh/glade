package vm

import "testing"

func TestGraphemeBoundaryTable(t *testing.T) {
	text := "e\u0301x"
	table := buildGraphemeBoundaryTable(text)
	if !table.isBoundaryByte(0) || !table.isBoundaryByte(len("e\u0301")) || !table.isBoundaryByte(len(text)) {
		t.Fatalf("missing expected byte boundaries: %#v", table)
	}
	if table.isBoundaryByte(len("e")) {
		t.Fatalf("boundary table split combining sequence")
	}
}

func TestCompileGraphemeRegexPlanMatchesExtendedClusters(t *testing.T) {
	text := "👍🏽x"
	plan, err := compileRegexp2PlanForInput("Pattern.compile", `\X`, text)
	if err != nil {
		t.Fatal(err)
	}
	match, err := plan.findValidStartingAt(text, 0)
	if err != nil {
		t.Fatal(err)
	}
	if match == nil {
		t.Fatal("expected grapheme match")
	}
	indices, err := plan.matchByteIndices(text, match, 0)
	if err != nil {
		t.Fatal(err)
	}
	if got := text[indices[0]:indices[1]]; got != "👍🏽" {
		t.Fatalf("group = %q, want thumbs-up skin-tone cluster", got)
	}
}

func TestCompileGraphemeRegexPlanMatchesGraphemeBoundaries(t *testing.T) {
	text := "e\u0301x"
	plan, err := compileRegexp2PlanForInput("Pattern.compile", `\b{g}`, text)
	if err != nil {
		t.Fatal(err)
	}
	var starts []int
	match, err := plan.findValidStartingAt(text, 0)
	for match != nil && err == nil {
		indices, err := plan.matchByteIndices(text, match, 0)
		if err != nil {
			t.Fatal(err)
		}
		start, err := apexStringIndexForByteIndex(text, indices[0])
		if err != nil {
			t.Fatal(err)
		}
		starts = append(starts, start)
		match, err = plan.findNextValid(text, match)
	}
	if err != nil {
		t.Fatal(err)
	}
	want := []int{0, 2, 3}
	if len(starts) != len(want) {
		t.Fatalf("boundary starts = %v, want %v", starts, want)
	}
	for i := range starts {
		if starts[i] != want[i] {
			t.Fatalf("boundary starts = %v, want %v", starts, want)
		}
	}
}
