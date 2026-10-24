package vm

import "testing"

// Native regex_conservative_split/org.tsv:8; ORG-RUN.txt.
func TestLiteralSplitEligibilityPlanRoutePreservesPrivateGraphemeAssertions(t *testing.T) {
	for _, expression := range []string{`e\b{g}`, `\X`} {
		t.Run(expression, func(t *testing.T) {
			plan, err := compileRegexp2PlanForInput("String.split", expression, "e\u0301")
			if err != nil {
				t.Fatal(err)
			}
			if len(plan.internalGroupNames) == 0 {
				t.Fatal("control pattern did not produce a private grapheme assertion")
			}
			if parts, ok := splitLiteralRegexPlan(plan, "e\u0301", -1); ok {
				t.Fatalf("private assertion entered literal path: %q", parts)
			}
		})
	}
	plan, err := compileRegexp2PlanForInput("String.split", `\.`, "0.0.0.0")
	if err != nil {
		t.Fatal(err)
	}
	if parts, ok := splitLiteralRegexPlan(plan, "0.0.0.0", -1); ok {
		t.Fatalf("escaped dot entered literal path: %q eligible=%v", parts, ok)
	}
}
