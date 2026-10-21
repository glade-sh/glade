package sema

import "testing"

// String.contains returns Boolean, regardless of parentheses in its String
// argument. Quoted text and comments do not delimit method argument lists.
// https://developer.salesforce.com/docs/atlas.en-us.apexref.meta/apexref/apex_methods_system_string.htm
func TestStringPredicateCallArguments(t *testing.T) {
	for _, tc := range []struct {
		name, expression string
		reject           bool
	}{
		{"opening parenthesis", `value.toUpperCase().contains('COUNT(')`, false},
		{"implicit call chain", `toString().contains('SELECT * FROM (VALUES')`, false},
		{"closing parenthesis", `value.contains(')')`, false},
		{"escaped quote", `value.contains('\'(')`, false},
		{"comment parenthesis", `value.contains(/* ( */ 'x')`, false},
		{"nested call", `value.contains(value.substring(0, 1))`, false},
		{"plain predicate control", `value.contains('x')`, false},
		{"string argument control", `value.toUpperCase()`, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Phrase.cls": `public class Phrase {
 public void run(String value) { Assert.isTrue(` + tc.expression + `); }
 public override String toString() { return 'text'; }
}`}, "67.0")
			if tc.reject {
				if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA023") {
					t.Fatalf("String argument was accepted: %#v", result.Diagnostics)
				}
			} else if result.HasErrors() {
				t.Fatalf("Boolean predicate was rejected: %#v", result.Diagnostics)
			}
		})
	}
}
