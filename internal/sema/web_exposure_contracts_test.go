package sema

import (
	"strings"
	"testing"
)

func TestRestExposureContracts(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"owner must be global":             `@RestResource(urlMapping='/items') public class Probe { @HttpGet global static void get() {} }`,
		"mapping must lead slash":          `@RestResource(urlMapping='items') global class Probe { @HttpGet global static void get() {} }`,
		"mapping wildcard slash delimited": `@RestResource(urlMapping='/items*detail') global class Probe { @HttpGet global static void get() {} }`,
		"mapping must be literal":          `@RestResource(urlMapping='/items' + '/detail') global class Probe { @HttpGet global static void get() {} }`,
		"mapping has 255 character limit":  `@RestResource(urlMapping='/` + strings.Repeat("x", 255) + `') global class Probe { @HttpGet global static void get() {} }`,
		"get is no argument":               `@RestResource(urlMapping='/items') global class Probe { @HttpGet global static void get(String id) {} }`,
		"delete is no argument":            `@RestResource(urlMapping='/items') global class Probe { @HttpDelete global static void remove(String id) {} }`,
		"method must be static":            `@RestResource(urlMapping='/items') global class Probe { @HttpPost global void post() {} }`,
		"method must be global":            `@RestResource(urlMapping='/items') global class Probe { @HttpPost public static void post() {} }`,
		"one method per verb":              `@RestResource(urlMapping='/items') global class Probe { @HttpPost global static void one() {} @HttpPost global static void two() {} }`,
		"reject map signature":             `@RestResource(urlMapping='/items') global class Probe { @HttpPost global static Map<String,String> post() { return null; } }`,
		"reject set signature":             `@RestResource(urlMapping='/items') global class Probe { @HttpPost global static void post(Set<String> values) {} }`,
		"reject blob signature":            `@RestResource(urlMapping='/items') global class Probe { @HttpPost global static Blob post() { return null; } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA033") {
				t.Fatalf("expected REST contract: %#v", result.Diagnostics)
			}
		})
	}
}

func TestRestVerbRequiresRestResourceOwner(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `global class Probe { @HttpGet global static void get() {} }`,
	})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA026") {
		t.Fatalf("HTTP verb outside RestResource owner was accepted: %#v", result.Diagnostics)
	}
}

func TestRestExposureAllowsMultipleMethodAnnotations(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": `@RestResource(urlMapping='/items/*/detail') global class Probe { @HttpGet @HttpPost global static void run() {} }`})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA033") {
		t.Fatalf("unexpected REST contract: %#v", result.Diagnostics)
	}
}

func TestRestExposureAllowsSlashDelimitedNonterminalWildcard(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"Probe.cls": `@RestResource(urlMapping='/items/*/detail') global class Probe { @HttpGet global static void get() {} }`,
	})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA033") {
		t.Fatalf("slash-delimited nonterminal wildcard was rejected: %#v", result.Diagnostics)
	}
}

func TestSOAPExposureContracts(t *testing.T) {
	t.Parallel()
	for name, source := range map[string]string{
		"owner must be global":  `public class Probe { webservice static String run() { return 'ok'; } }`,
		"owner cannot be inner": `global class Outer { global class Probe { webservice static String run() { return 'ok'; } } }`,
		"method must be static": `global class Probe { webservice String run() { return 'ok'; } }`,
		"reject map":            `global class Probe { webservice static Map<String,String> run() { return null; } }`,
		"reject set":            `global class Probe { webservice static void run(Set<String> values) {} }`,
		"reject blob":           `global class Probe { webservice static Blob run() { return null; } }`,
		"reject overload":       `global class Probe { webservice static String run() { return 'ok'; } webservice static String run(String value) { return value; } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA033") {
				t.Fatalf("expected SOAP contract: %#v", result.Diagnostics)
			}
		})
	}
}

func TestSOAPExposureAllowsLoggingLevel(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": `global class Probe { webservice static System.LoggingLevel run(System.LoggingLevel value) { return value; } }`})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA033") {
		t.Fatalf("unexpected SOAP contract: %#v", result.Diagnostics)
	}
}

func TestSOAPExposureAllowsNestedGlobalWrapperProperties(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": `
global class Probe {
    global class Wrapper {
        webservice String value { get; set; }
        webservice List<String> values { get; set; }
    }
}`})
	if hasDiagnosticCode(result.Diagnostics, "GLADESEMA033") {
		t.Fatalf("nested global SOAP wrapper properties were rejected: %#v", result.Diagnostics)
	}
}

func TestSOAPExposureRejectsUnsupportedWrapperProperties(t *testing.T) {
	for name, source := range map[string]string{
		"map":  `global class Probe { global class Wrapper { webservice Map<String, String> value { get; set; } } }`,
		"set":  `global class Probe { global class Wrapper { webservice Set<String> value { get; set; } } }`,
		"blob": `global class Probe { global class Wrapper { webservice Blob value { get; set; } } }`,
	} {
		t.Run(name, func(t *testing.T) {
			result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": source})
			if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA033") {
				t.Fatalf("unsupported SOAP wrapper property was accepted: %#v", result.Diagnostics)
			}
		})
	}
}

func TestSOAPExposureRejectsNonGlobalWrapperProperties(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": `
global class Probe {
    public class Wrapper {
        webservice String value { get; set; }
    }
}`})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA033") {
		t.Fatalf("non-global SOAP wrapper property was accepted: %#v", result.Diagnostics)
	}
}

func TestSOAPExposureRejectsTopLevelProperties(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{"Probe.cls": `
global class Probe {
    webservice String value { get; set; }
}`})
	if !hasDiagnosticCode(result.Diagnostics, "GLADESEMA033") {
		t.Fatalf("top-level SOAP property was accepted: %#v", result.Diagnostics)
	}
}
