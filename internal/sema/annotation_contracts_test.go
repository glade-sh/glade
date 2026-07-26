package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/apexast"
)

func TestAnnotationCatalogRejectsUnknownAnnotationsAndProperties(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"unknown annotation": `@DoesNotExist public class Probe {}`,
		"unknown property":   `public class Probe { @AuraEnabled(doesNotExist=true) public static void run() {} }`,
		"preview annotation": `@IntegrationTest public class Probe {}`,
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

func TestAnnotationCatalogAllowsSuppressWarnings(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `@SuppressWarnings('PMD.EmptyStatementBlock') public class Probe { public static void run() {} }`,
	})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA031") {
		t.Fatalf("unexpected catalog diagnostic: %#v", result.Diagnostics)
	}
}

func TestAnnotationContractsRejectInvalidOwnersAndSignatures(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"test method outside test class":            `public class Probe { @IsTest static void run() {} }`,
		"test method requires static no args":       `@IsTest private class Probe { @IsTest void run(String value) {} }`,
		"duplicate test setup":                      `@IsTest private class Probe { @TestSetup static void one() {} @TestSetup static void two() {} }`,
		"aura enabled overload":                     `public class Probe { @AuraEnabled public static void run() {} @AuraEnabled public static void run(String value) {} }`,
		"invocable method requires list parameter":  `public class Probe { @InvocableMethod public static void run(String value) {} }`,
		"invocable variable cannot be static":       `public class Probe { @InvocableVariable public static String value; }`,
		"remote action requires public static":      `public class Probe { @RemoteAction private void run() {} }`,
		"ReadOnly static method needs a web owner":  `public class Probe { @ReadOnly public static void run() {} }`,
		"JsonAccess requires a parameter":           `@JsonAccess public class Probe {}`,
		"JsonAccess cannot annotate a method":       `public class Probe { @JsonAccess(serializable=true) public void run() {} }`,
		"NamespaceAccessible cannot be AuraEnabled": `public class Probe { @AuraEnabled @NamespaceAccessible public static void run() {} }`,
		"NamespaceAccessible member needs owner":    `public class Probe { @NamespaceAccessible public void run() {} }`,
		"InvocableMethod allows no annotation mix":  `public class Probe { @future @InvocableMethod public static void run(List<String> values) {} }`,
		"InvocableVariable cannot use Object":       `public class Probe { @InvocableVariable public Object value; }`,
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
