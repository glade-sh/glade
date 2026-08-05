package sema

import "testing"

func TestCB339ScalarConstructorsMatchSalesforce(t *testing.T) {
	for _, typeName := range []string{"Date", "Datetime", "Decimal", "Double"} {
		t.Run(typeName, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Probe.cls": "public class Probe { public void run() { " + typeName + " value = new " + typeName + "(); } }",
			}, "67.0")
			if !result.HasErrors() {
				t.Fatalf("new %s should be rejected as non-constructable", typeName)
			}
			if len(result.Diagnostics) == 0 || result.Diagnostics[0].Code != "GLADESEMA028" {
				t.Fatalf("new %s diagnostics = %#v, want unsupported-constructor diagnostic", typeName, result.Diagnostics)
			}
		})
	}
}
