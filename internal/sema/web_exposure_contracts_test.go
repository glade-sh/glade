package sema

import "testing"

func TestRestExposureContracts(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"owner must be global":    `@RestResource(urlMapping='/items') public class Probe { @HttpGet global static void get() {} }`,
		"mapping must lead slash": `@RestResource(urlMapping='items') global class Probe { @HttpGet global static void get() {} }`,
		"get is no argument":      `@RestResource(urlMapping='/items') global class Probe { @HttpGet global static void get(String id) {} }`,
		"one method per verb":     `@RestResource(urlMapping='/items') global class Probe { @HttpPost global static void one() {} @HttpPost global static void two() {} }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA033") {
				t.Fatalf("expected REST contract: %#v", result.Diagnostics)
			}
		})
	}
}

func TestRestExposureAllowsMultipleMethodAnnotations(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": `@RestResource(urlMapping='/items/*') global class Probe { @HttpGet @HttpPost global static void run() {} }`})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA033") {
		t.Fatalf("unexpected REST contract: %#v", result.Diagnostics)
	}
}
