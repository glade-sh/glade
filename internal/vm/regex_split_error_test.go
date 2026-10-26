package vm

import (
	"reflect"
	"strings"
	"testing"
)

// No native row covers a split whose first search exhausts the regex budget,
// so that path keeps the base runtime's result: the input comes back unsplit
// and no error is raised. The premature-timeout retry must not change it,
// because a search that spent its budget is never retried. This checks
// Glade's local engine budget, not Salesforce's regex work limit.
func TestRegexSplitFirstSearchBudgetKeepsBaseResultStringAndPatternRoutes(t *testing.T) {
	const pattern = `(0+)+x`
	text := strings.Repeat("0", 40)
	compiled, err := patternCompile([]Value{String(pattern)})
	if err != nil {
		t.Fatal(err)
	}
	for _, route := range []string{"String.split", "Pattern.split/source", "Pattern.split/value"} {
		t.Run(route, func(t *testing.T) {
			var parts []string
			var err error
			switch route {
			case "String.split":
				parts, err = stringRegexSplit(text, []Value{String(pattern)})
			case "Pattern.split/source":
				parts, err = patternSplit(pattern, []Value{String(text)})
			case "Pattern.split/value":
				parts, err = patternSplitValue(compiled, []Value{String(text)})
			}
			if err != nil || !reflect.DeepEqual(parts, []string{text}) {
				t.Fatalf("first-search budget result changed from base: parts=%q error=%v", parts, err)
			}
		})
	}
}
