package vm

import (
	"reflect"
	"testing"
)

// numeric_routes/org.tsv, grapheme-boundary: API 62/67 and anonymous/isTest.
// Private grapheme captures remain assertions even when the rewrite is literal.
func TestLiteralSplitPreservesGraphemePlanStringAndPatternRoutes(t *testing.T) {
	text := "e\u0301"
	expression := `e\b{g}`
	plan, err := compileRegexp2PlanForInput("Pattern.split", expression, text)
	if err != nil {
		t.Fatal(err)
	}
	match, err := plan.findValidStartingAt(text, 0)
	if err != nil || match != nil {
		t.Fatalf("matcher-plan control: match=%v error=%v", match, err)
	}
	pattern, err := patternCompile([]Value{String(expression)})
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"String.split", "Pattern.split/source", "Pattern.split/value"} {
		t.Run(route, func(t *testing.T) {
			var parts []string
			var err error
			switch route {
			case "String.split":
				parts, err = stringRegexSplit(text, []Value{String(expression), Int(-1)})
			case "Pattern.split/source":
				parts, err = patternSplit(expression, []Value{String(text), Int(-1)})
			case "Pattern.split/value":
				parts, err = patternSplitValue(pattern, []Value{String(text), Int(-1)})
			}
			if err != nil || !reflect.DeepEqual(parts, []string{text}) {
				t.Fatalf("split bypassed matcher-plan boundary validation: parts=%q error=%v", parts, err)
			}
		})
	}
}
