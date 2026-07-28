package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
)

func TestAnnotationCatalogRejectsUnknownAnnotationsAndProperties(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"unknown annotation":  `@DoesNotExist public class Probe {}`,
		"webservice modifier": `@webservice public class Probe {}`,
		"unknown property":    `public class Probe { @AuraEnabled(doesNotExist=true) public static void run() {} }`,
		"unexpected positional argument": `@RestResource(urlMapping='/probe') global class Probe {
			@HttpGet('unexpected') global static void run() {}
		}`,
		"non-string suppress warnings argument": `@SuppressWarnings(1) public class Probe {}`,
		"duplicate property":                    `public class Probe { @AuraEnabled(cacheable=true cacheable=false) public static void run() {} }`,
		"InvocableMethod string property":       `public class Probe { @InvocableMethod(label=true) public static void run(List<String> values) {} }`,
		"InvocableMethod boolean property":      `public class Probe { @InvocableMethod(callout='yes') public static void run(List<String> values) {} }`,
		"InvocableVariable boolean property":    `public class Probe { @InvocableVariable(required='true') public String value; }`,
		"preview annotation":                    `@IntegrationTest public class Probe {}`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA031") {
				t.Fatalf("expected annotation catalog diagnostic, got %#v", result.Diagnostics)
			}
		})
	}
}

func TestAnnotationCatalogChecksParametersAndAccessors(t *testing.T) {
	for name, source := range map[string]string{
		"parameter": `public class Probe { public void run(@DoesNotExist String value) {} }`,
		"accessor":  `public class Probe { public String Value { @DoesNotExist get; set; } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA031") {
				t.Fatalf("unknown %s annotation was accepted: %#v", name, result.Diagnostics)
			}
		})
	}
}

func TestAnnotationCatalogAllowsKnownProperties(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `@IsTest(SeeAllData=false) private class Probe { @AuraEnabled(cacheable=true scope='global') public static String run() { return 'ok'; } }`,
	})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA031") {
		t.Fatalf("unexpected catalog diagnostic: %#v", result.Diagnostics)
	}
}

func TestJsonAccessAllowsDocumentedStringModes(t *testing.T) {
	for _, mode := range []string{"always", "sameNamespace", "samePackage", "never"} {
		t.Run(mode, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{
				"Probe.cls": "@JsonAccess(serializable='" + mode + "' deserializable='" + mode + "') public class Probe {}",
			})
			if result.HasErrors() {
				t.Fatalf("JsonAccess mode %q was rejected: %#v", mode, result.Diagnostics)
			}
		})
	}
}

func TestJsonAccessRejectsUnknownStringModeOnceForNestedType(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe {
  @JsonAccess(serializable='sometimes')
  public class Nested {}
}`,
	})
	count := 0
	for _, diagnostic := range result.Diagnostics {
		if diagnostic.Code == "GLADESEMA032" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("nested JsonAccess diagnostics = %d, want 1: %#v", count, result.Diagnostics)
	}
}

func TestAnnotationCatalogAllowsDocumentedInvocableMethodProperties(t *testing.T) {
	for name, properties := range map[string]string{
		"icon property":                `iconName='slds:standard:account'`,
		"capability property":          `capabilityType='PromptTemplateType://SalesEmail'`,
		"configurationEditor property": `configurationEditor='c-editor'`,
	} {
		t.Run(name, func(t *testing.T) {
			source := `
public class Probe {
  @InvocableMethod(` + properties + `)
  public static void run(List<String> values) {}
}
`
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if result.HasErrors() {
				t.Fatalf("documented InvocableMethod properties were rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestAnnotationCatalogAllowsDocumentedInvocableVariableProperties(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `
public class Probe {
  @InvocableVariable(defaultValue='hello' placeholderText='Enter a value')
  public String value;
  @InvocableVariable(placeholderText='true')
  public Boolean booleanValue;
  @InvocableVariable(placeholderText='1.5')
  public Decimal decimalValue;
  @InvocableVariable(placeholderText='10L')
  public Long longValue;
}
`,
	})
	if result.HasErrors() {
		t.Fatalf("documented InvocableVariable properties were rejected: %#v", result.Diagnostics)
	}
}

func TestInvocableVariablePropertyContracts(t *testing.T) {
	for name, source := range map[string]string{
		"default with required":     `public class Probe { @InvocableVariable(defaultValue='value' required=true) public String value; }`,
		"default unsupported type":  `public class Probe { @InvocableVariable(defaultValue='2026-01-01') public Date value; }`,
		"Boolean default":           `public class Probe { @InvocableVariable(defaultValue='sometimes') public Boolean value; }`,
		"Double default suffix":     `public class Probe { @InvocableVariable(defaultValue='1.5') public Double value; }`,
		"Integer default shape":     `public class Probe { @InvocableVariable(defaultValue='1.5') public Integer value; }`,
		"Long default suffix":       `public class Probe { @InvocableVariable(defaultValue='10') public Long value; }`,
		"placeholder unsupported":   `public class Probe { @InvocableVariable(placeholderText='2026-01-01') public Date value; }`,
		"Double placeholder suffix": `public class Probe { @InvocableVariable(placeholderText='1.5') public Double value; }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA032") {
				t.Fatalf("invalid InvocableVariable property was accepted: %#v", result.Diagnostics)
			}
		})
	}

	for name, source := range map[string]string{
		"Boolean":                  `public class Probe { @InvocableVariable(defaultValue='TRUE' placeholderText='false') public Boolean value; }`,
		"Decimal":                  `public class Probe { @InvocableVariable(defaultValue='1.5' placeholderText='2.5') public Decimal value; }`,
		"Double":                   `public class Probe { @InvocableVariable(defaultValue='1.5D' placeholderText='2D') public Double value; }`,
		"Integer":                  `public class Probe { @InvocableVariable(defaultValue='-1' placeholderText='2') public Integer value; }`,
		"Long":                     `public class Probe { @InvocableVariable(defaultValue='10L' placeholderText='20L') public Long value; }`,
		"String":                   `public class Probe { @InvocableVariable(defaultValue='' placeholderText='Enter a value') public String value; }`,
		"System.String":            `public class Probe { @InvocableVariable(defaultValue='value') public System.String value; }`,
		"System.Integer":           `public class Probe { @InvocableVariable(defaultValue='1') public System.Integer value; }`,
		"required without default": `public class Probe { @InvocableVariable(required=true) public String value; }`,
	} {
		t.Run("valid "+name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if result.HasErrors() {
				t.Fatalf("valid InvocableVariable property was rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestInvocableMethodCapabilityTypeFormat(t *testing.T) {
	source := `public class Probe { @InvocableMethod(capabilityType='not a capability') public static void run(List<String> values) {} }`
	result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA032") {
		t.Fatalf("invalid capabilityType was accepted: %#v", result.Diagnostics)
	}
}

func TestIsTestClassAllowsCriticalAndTestForAtAPIVersion66(t *testing.T) {
	for name, property := range map[string]string{
		"critical": "critical=true",
		"testFor":  "testFor='ApexClass:Probe'",
	} {
		t.Run(name, func(t *testing.T) {
			source := "@IsTest(" + property + ") private class Probe {}"
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, "66.0")
			if result.HasErrors() {
				t.Fatalf("API 66 IsTest class property was rejected: %#v", result.Diagnostics)
			}
		})
	}
}

func TestIsTestClassRejectsCriticalAndTestForBeforeAPIVersion66(t *testing.T) {
	for name, property := range map[string]string{
		"critical": "critical=true",
		"testFor":  "testFor='ApexClass:Probe'",
	} {
		t.Run(name, func(t *testing.T) {
			source := "@IsTest(" + property + ") private class Probe {}"
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, "65.0")
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA032") {
				t.Fatalf("API 65 IsTest property was accepted: %#v", result.Diagnostics)
			}
		})
	}
}

func TestAnnotationCatalogAllowsSuppressWarnings(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `@SuppressWarnings('PMD.EmptyStatementBlock') public class Probe { public static void run() {} }`,
	})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA031") {
		t.Fatalf("unexpected catalog diagnostic: %#v", result.Diagnostics)
	}
}

func TestSuppressWarningsRequiresSingleStringArgument(t *testing.T) {
	for name, source := range map[string]string{
		"multiple arguments":  `@SuppressWarnings('one', 'two') public class Probe {}`,
		"non-string argument": `@SuppressWarnings(1) public class Probe {}`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !result.HasErrors() {
				t.Fatalf("invalid SuppressWarnings arguments were accepted: %#v", result.Diagnostics)
			}
		})
	}
}

func TestAnnotationContractsRejectInvalidOwnersAndSignatures(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"test method outside test class":           `public class Probe { @IsTest static void run() {} }`,
		"test method requires no args":             `@IsTest private class Probe { @IsTest static void run(String value) {} }`,
		"test method requires static":              `@IsTest private class Probe { @IsTest void run() {} }`,
		"duplicate test setup":                     `@IsTest private class Probe { @TestSetup static void one() {} @TestSetup static void two() {} }`,
		"aura enabled overload":                    `public class Probe { @AuraEnabled public static void run() {} @AuraEnabled public static void run(String value) {} }`,
		"invocable method requires list parameter": `public class Probe { @InvocableMethod public static void run(String value) {} }`,
		"invocable variable cannot be static":      `public class Probe { @InvocableVariable public static String value; }`,
		"remote action requires public":            `public class Probe { @RemoteAction private static void run() {} }`,
		"remote action requires static":            `public class Probe { @RemoteAction public void run() {} }`,
		"ReadOnly static method needs a web owner": `public class Probe { @ReadOnly public static void run() {} }`,
		"JsonAccess requires a parameter":          `@JsonAccess public class Probe {}`,
		"JsonAccess cannot annotate a method":      `public class Probe { @JsonAccess(serializable=true) public void run() {} }`,
		"InvocableMethod allows no annotation mix": `public class Probe { @future @InvocableMethod public static void run(List<String> values) {} }`,
		"InvocableVariable cannot use Object":      `public class Probe { @InvocableVariable public Object value; }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA032") {
				t.Fatalf("expected annotation contract diagnostic: %#v", result.Diagnostics)
			}
		})
	}
}

func TestAnnotationContractsRejectMultipleInvocableMethods(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { @InvocableMethod public static void first(List<String> values) {} @InvocableMethod public static void second(List<String> values) {} }`,
	})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA032") {
		t.Fatalf("multiple InvocableMethods accepted: %#v", result.Diagnostics)
	}
}

func TestAnnotationContractsAllowValueReturningTestMethod(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `@IsTest private class Probe { @IsTest static Integer run() { return 1; } }`,
	})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA032") {
		t.Fatalf("unexpected annotation contract diagnostic: %#v", result.Diagnostics)
	}
}

func TestAnnotationContractsGateMethodIsTestSeeAllDataAtAPIVersion24(t *testing.T) {
	for _, test := range []struct {
		apiVersion string
		wantError  bool
	}{
		{apiVersion: "23.0", wantError: true},
		{apiVersion: "24.0", wantError: false},
	} {
		t.Run(test.apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Probe.cls": `@IsTest private class Probe { @IsTest(SeeAllData=true) static void run() {} }`,
			}, test.apiVersion)
			if got := hasDiagnosticCode(result.Diagnostics, "GLADESEMA032"); got != test.wantError {
				t.Fatalf("API %s method IsTest(SeeAllData=true) error = %v, want %v: %#v", test.apiVersion, got, test.wantError, result.Diagnostics)
			}
		})
	}
}

func TestAnnotationContractsAllowMethodIsTestSeeAllDataWithoutEffectiveAPIVersion(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `@IsTest private class Probe { @IsTest(SeeAllData=true) static void run() {} }`,
	})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA032") {
		t.Fatalf("method IsTest(SeeAllData=true) without an effective API version was rejected: %#v", result.Diagnostics)
	}
}

func TestAnnotationContractsAllowZeroParameterInvocableMethod(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `public class Probe { @InvocableMethod public static void run() {} }`,
	})
	if result.HasErrors() {
		t.Fatalf("zero-parameter InvocableMethod was rejected: %#v", result.Diagnostics)
	}
}

func TestNamespaceAccessibleIsAPIVersionGatedAndAllowsPublicInterfaces(t *testing.T) {
	source := `@NamespaceAccessible public interface Probe { @NamespaceAccessible void run(); }`
	for _, test := range []struct {
		apiVersion string
		wantError  bool
	}{
		{apiVersion: "49.0", wantError: false},
		{apiVersion: "50.0", wantError: false},
	} {
		t.Run(test.apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, test.apiVersion)
			if got := hasDiagnosticCode(result.Diagnostics, "GLADESEMA032"); got != test.wantError {
				t.Fatalf("API %s NamespaceAccessible error = %v, want %v: %#v", test.apiVersion, got, test.wantError, result.Diagnostics)
			}
		})
	}
}

func TestNamespaceAccessibleMemberIsAPIVersionGatedOnGlobalOwner(t *testing.T) {
	source := `global class Probe { @NamespaceAccessible global void run() {} }`
	for _, test := range []struct {
		apiVersion string
		wantError  bool
	}{
		{apiVersion: "49.0", wantError: false},
		{apiVersion: "50.0", wantError: false},
	} {
		t.Run(test.apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, test.apiVersion)
			if got := hasDiagnosticCode(result.Diagnostics, "GLADESEMA032"); got != test.wantError {
				t.Fatalf("API %s NamespaceAccessible member error = %v, want %v: %#v", test.apiVersion, got, test.wantError, result.Diagnostics)
			}
		})
	}
}

func TestNamespaceAccessibleSkipsContractsWithoutAnEffectiveAPIVersion(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `@NamespaceAccessible public class Probe { @NamespaceAccessible public static void run() {} }`,
	})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA032") {
		t.Fatalf("NamespaceAccessible source without an effective API version was rejected: %#v", result.Diagnostics)
	}
}

func TestNamespaceAccessibleRejectsInvalidOwnersAtAPIVersion50(t *testing.T) {
	for name, source := range map[string]string{
		"private type":                  `@NamespaceAccessible private class Probe {}`,
		"member no owner":               `public class Probe { @NamespaceAccessible public void run() {} }`,
		"member mixed with AuraEnabled": `public class Probe { @AuraEnabled @NamespaceAccessible public static void run() {} }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, "50.0")
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA032") {
				t.Fatalf("API 50 invalid NamespaceAccessible %s was accepted: %#v", name, result.Diagnostics)
			}
		})
	}
}

func TestFutureParametersRequirePrimitiveValues(t *testing.T) {
	for _, test := range []struct {
		name    string
		typ     string
		allowed bool
	}{
		{name: "primitive", typ: "Id", allowed: true},
		{name: "primitive array", typ: "String[]", allowed: true},
		{name: "primitive list", typ: "List<Id>", allowed: true},
		{name: "primitive map", typ: "Map<Id, String>", allowed: true},
		{name: "sobject list", typ: "List<Account>", allowed: false},
		{name: "sobject map value", typ: "Map<Id, Account>", allowed: false},
		{name: "object list", typ: "List<Object>", allowed: false},
		{name: "nested collection", typ: "List<List<Id>>", allowed: false},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := futureParametersAllowed([]apexast.Parameter{{Type: test.typ}})
			if got != test.allowed {
				t.Fatalf("future parameter %s allowed = %v, want %v", test.typ, got, test.allowed)
			}
		})
	}
}

func TestAnnotationContractsRejectInvalidTargetsAndValues(t *testing.T) {
	for name, source := range map[string]string{
		"AuraEnabled class target":        `@AuraEnabled public class Probe {}`,
		"AuraEnabled positional value":    `public class Probe { @AuraEnabled('x') public static void run() {} }`,
		"AuraEnabled nonboolean property": `public class Probe { @AuraEnabled(cacheable='yes') public static void run() {} }`,
		"ReadOnly REST method needs verb": `@RestResource(urlMapping='/probe') global class Probe { @ReadOnly global static void run() {} }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA032") && !hasDiagnosticCode(result.Diagnostics, "GLADESEMA033") {
				t.Fatalf("invalid annotation contract was accepted: %#v", result.Diagnostics)
			}
		})
	}
}
