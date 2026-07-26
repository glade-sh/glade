package sema

import "testing"

func TestTypeContractRejectsInvalidSourceTypesAndLiterals(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"currency source type":      `public class Probe { public Currency value; }`,
		"raw list construction":     `public class Probe { public void run() { System.debug(new List()); } }`,
		"raw map construction":      `public class Probe { public void run() { System.debug(new Map()); } }`,
		"list generic arity":        `public class Probe { public List<String, Integer> values; }`,
		"map generic arity":         `public class Probe { public Map<String> values; }`,
		"collection depth":          `public class Probe { public List<List<List<List<List<List<List<List<List<String>>>>>>>>> values; }`,
		"integer overflow":          `public class Probe { public void run() { System.debug(2147483648); } }`,
		"minimum integer magnitude": `public class Probe { public Integer value() { return -2147483648; } }`,
		"cast minimum magnitude":    `public class Probe { public Integer value() { return (Integer) -2147483648; } }`,
		"binary integer overflow":   `public class Probe { public void run() { System.debug(0 - 2147483648); } }`,
		"postfix binary overflow":   `public class Probe { public void run() { Integer value = 0; System.debug(value++ - 2147483648); } }`,
		"scientific notation":       `public class Probe { public void run() { System.debug(1e3); } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA019") {
				t.Fatalf("expected source contract diagnostic, got %#v", result.Diagnostics)
			}
		})
	}
}

func TestTypeContractIgnoresNumericTextInStringsAndComments(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": `
public class Probe {
  public void run() {
    String salesforceId = '003000000000001';
    String exponent = '1e3';
    // 2147483648 and 1e3 are not source literals.
  }
}
`})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA019") {
		t.Fatalf("numeric text outside code was rejected: %#v", result.Diagnostics)
	}
}

func TestTypeContractAllowsLargeDecimalLiteral(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public Decimal value() { return 2147483648.0; } }`,
	})
	if result.HasErrors() {
		t.Fatalf("large Decimal literal was rejected: %#v", result.Diagnostics)
	}
}

func TestTypeContractAllowsLongSuffixLiteral(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { public Long value() { return 2147483648L; } }`,
	})
	if result.HasErrors() {
		t.Fatalf("Long suffix literal was rejected: %#v", result.Diagnostics)
	}
}

func TestTypeContractRejectsRawCollectionConstructorsCaseInsensitively(t *testing.T) {
	for _, constructor := range []string{"lIsT", "mAp", "SET"} {
		t.Run(constructor, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"Probe.cls": "public class Probe { public void run() { System.debug(new " + constructor + "()); } }",
			})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA019") {
				t.Fatalf("raw %s constructor was accepted: %#v", constructor, result.Diagnostics)
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

func TestTypeContractAllowsIterableInstanceofAtAllTestedAPIVersions(t *testing.T) {
	for _, apiVersion := range []string{"59.0", "60.0"} {
		t.Run(apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Probe.cls": `public class Probe { public Boolean run(List<Account> values) { return values instanceof Iterable<SObject>; } }`,
			}, apiVersion)
			if result.HasErrors() {
				t.Fatalf("Iterable instanceof control was rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestTypeContractRejectsAlwaysTrueNestedIterableInstanceofAtAPIVersion60(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"Probe.cls": `public class Probe { public virtual class Base {} public class Sub extends Base {} public Boolean run() { List<Sub> values = new List<Sub>(); return values instanceof Iterable<Base>; } }`,
	}, "60.0")
	if !result.HasErrors() || !declarationDiagnosticMatching(result, "always true") {
		t.Fatalf("always-true nested Iterable instanceof was accepted: %#v", result.Diagnostics)
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

func TestTypeContractAcceptsSafeNavigationEqualityRead(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": `
public class Probe {
  public Boolean label;
  public void run(Probe value) {
    if (value?.label == true) {}
  }
}`})
	if result.HasErrors() {
		t.Fatalf("safe-navigation equality read was rejected: %#v", result.Diagnostics)
	}
}
