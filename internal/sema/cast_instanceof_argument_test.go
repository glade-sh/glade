package sema

import "testing"

// Apex's operator precedence places a type cast before instanceof, so the
// comparison's result is Boolean even when its left operand is cast to Object.
// https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/langCon_apex_expressions_operators_precedence.htm
func TestCastInstanceOfMethodArguments(t *testing.T) {
	for _, test := range []struct {
		name       string
		expression string
		wantReject bool
	}{
		{name: "cast comparison", expression: `(Object) value instanceof Integer`},
		{name: "grouped cast comparison", expression: `((Object) value) instanceof Integer`},
		{name: "negated comparison", expression: `!((Object) value instanceof Integer)`},
		{name: "conditional comparison", expression: `flag ? (Object) value instanceof Integer : false`},
		{name: "object argument control", expression: `(Object) value`, wantReject: true},
	} {
		t.Run(test.name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"Probe.cls": `public class Probe {
  public static void run(Object value, Boolean flag) {
    System.Assert.isTrue(` + test.expression + `);
  }
}`,
			})
			if test.wantReject {
				if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA023") {
					t.Fatalf("expected invalid Object argument diagnostic: %#v", result.Diagnostics)
				}
			} else if result.HasErrors() {
				t.Fatalf("Boolean comparison argument rejected: %#v", result.Diagnostics)
			}
		})
	}
}
