package sema

import "testing"

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

func TestAnnotationCatalogAllowsKnownProperties(t *testing.T) {
	t.Parallel()
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `@IsTest(SeeAllData=false) private class Probe { @AuraEnabled(cacheable=true) public static String run() { return 'ok'; } }`,
	})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA031") {
		t.Fatalf("unexpected catalog diagnostic: %#v", result.Diagnostics)
	}
}

func TestAnnotationContractsRejectInvalidOwnersAndSignatures(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"test method outside test class":           `public class Probe { @IsTest static void run() {} }`,
		"test method requires static no args":      `@IsTest private class Probe { @IsTest void run(String value) {} }`,
		"duplicate test setup":                     `@IsTest private class Probe { @TestSetup static void one() {} @TestSetup static void two() {} }`,
		"aura enabled overload":                    `public class Probe { @AuraEnabled public static void run() {} @AuraEnabled public static void run(String value) {} }`,
		"invocable method requires list parameter": `public class Probe { @InvocableMethod public static void run(String value) {} }`,
		"invocable variable cannot be static":      `public class Probe { @InvocableVariable public static String value; }`,
		"remote action requires public static":     `public class Probe { @RemoteAction private void run() {} }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA032") {
				t.Fatalf("expected annotation contract diagnostic: %#v", result.Diagnostics)
			}
		})
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
