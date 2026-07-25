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
