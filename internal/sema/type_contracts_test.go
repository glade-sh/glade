package sema

import "testing"

func TestTypeContractRejectsInvalidSourceTypesAndLiterals(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"currency source type":  `public class Probe { public Currency value; }`,
		"raw list construction": `public class Probe { public void run() { System.debug(new List()); } }`,
		"raw map construction":  `public class Probe { public void run() { System.debug(new Map()); } }`,
		"list generic arity":    `public class Probe { public List<String, Integer> values; }`,
		"map generic arity":     `public class Probe { public Map<String> values; }`,
		"collection depth":      `public class Probe { public List<List<List<List<List<List<List<List<List<String>>>>>>>>> values; }`,
		"integer overflow":      `public class Probe { public void run() { System.debug(2147483648); } }`,
		"scientific notation":   `public class Probe { public void run() { System.debug(1e3); } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA019") {
				t.Fatalf("expected source contract diagnostic, got %#v", result.Diagnostics)
			}
		})
	}
}

func TestTypeContractRejectsInvalidOperators(t *testing.T) {
	t.Parallel()
	for name, expression := range map[string]string{
		"negates integer":          "!1",
		"orders booleans":          "true < false",
		"multiplies string":        "'value' * 2",
		"casts incompatible types": "(String) 1",
		"impossible instanceof":    "1 instanceof String",
		"incompatible coalesce":    "'value' ?? 1",
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"Probe.cls": "public class Probe { public void run() { System.debug(" + expression + "); } }",
			})
			if !result.HasErrors() {
				t.Fatalf("expected expression contract diagnostic for %s, got %#v", expression, result.Diagnostics)
			}
		})
	}
}

func TestTypeContractAllowsNumericWideningAndCompatibleCasts(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  public void run() {
    Decimal widened = 1;
    System.debug((Object) 'value');
    System.debug(1.0 + 2);
  }
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("unexpected compatible expression diagnostic: %#v", result.Diagnostics)
	}
}

func TestTypeContractRejectsPropertyCapabilityAndSafeNavigationMisuse(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"writes get-only property":          `public class Probe { public String ReadOnly { get; } public void run() { ReadOnly = 'value'; } }`,
		"reads set-only property":           `public class Probe { public String WriteOnly { set; } public void run() { System.debug(WriteOnly); } }`,
		"safe navigation static receiver":   `public class Probe { public static String label; public void run() { System.debug(Probe?.label); } }`,
		"safe navigation assignment target": `public class Probe { public String label; public void run(Probe value) { value?.label = 'value'; } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA019") {
				t.Fatalf("expected property/safe-navigation contract diagnostic, got %#v", result.Diagnostics)
			}
		})
	}
}
