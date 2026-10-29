package vm

import "testing"

// A pattern without grapheme tokens compiles once and is shared between
// inputs; a grapheme pattern still gets a table built for its input.
func TestRegexp2PlanCacheSharesInputIndependentPlans(t *testing.T) {
	first, err := compileRegexp2PlanForInput("Pattern.compile", `a+b`, "aab")
	if err != nil {
		t.Fatal(err)
	}
	second, err := compileRegexp2PlanForInput("Pattern.compile", `a+b`, "xyz aaab")
	if err != nil {
		t.Fatal(err)
	}
	if first != second {
		t.Fatal("input-independent plan was compiled twice")
	}
	if first.grapheme != nil {
		t.Fatal("input-independent plan built a grapheme table")
	}
	grapheme, err := compileRegexp2PlanForInput("Pattern.compile", `\X`, "é")
	if err != nil {
		t.Fatal(err)
	}
	if grapheme.grapheme == nil || len(grapheme.grapheme.clusters) != 1 {
		t.Fatalf("grapheme plan table = %+v, want one cluster", grapheme.grapheme)
	}
	other, err := compileRegexp2PlanForInput("Pattern.compile", `\X`, "ab")
	if err != nil {
		t.Fatal(err)
	}
	if other == grapheme {
		t.Fatal("grapheme plan was shared between inputs")
	}
}
