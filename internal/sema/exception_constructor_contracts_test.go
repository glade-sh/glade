package sema

import "testing"

func TestAPI67StandardExceptionConstructorVisibility(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name       string
		arguments  string
		wantErrors bool
	}{
		{name: "Exception", arguments: "", wantErrors: true},
		{name: "Exception", arguments: "'message'", wantErrors: true},
		{name: "Exception", arguments: "cause", wantErrors: true},
		{name: "Exception", arguments: "'message', cause", wantErrors: true},
		{name: "InvalidParameterValueException", arguments: "", wantErrors: true},
		{name: "InvalidParameterValueException", arguments: "'message'", wantErrors: true},
		{name: "InvalidParameterValueException", arguments: "cause", wantErrors: true},
		{name: "InvalidParameterValueException", arguments: "'message', cause", wantErrors: true},
		{name: "NoAccessException", arguments: "", wantErrors: false},
		{name: "NoAccessException", arguments: "'message'", wantErrors: true},
		{name: "NoAccessException", arguments: "cause", wantErrors: true},
		{name: "NoAccessException", arguments: "'message', cause", wantErrors: true},
		{name: "NoDataFoundException", arguments: "", wantErrors: false},
		{name: "NoDataFoundException", arguments: "'message'", wantErrors: true},
		{name: "NoDataFoundException", arguments: "cause", wantErrors: true},
		{name: "NoDataFoundException", arguments: "'message', cause", wantErrors: true},
		{name: "NullPointerException", arguments: "", wantErrors: false},
		{name: "NullPointerException", arguments: "'message'", wantErrors: true},
		{name: "NullPointerException", arguments: "cause", wantErrors: true},
		{name: "NullPointerException", arguments: "'message', cause", wantErrors: true},
	}
	for _, test := range tests {
		for _, qualified := range []bool{false, true} {
			typeName := test.name
			if qualified {
				typeName = "System." + typeName
			}
			t.Run(typeName+"/"+test.arguments, func(t *testing.T) {
				source := "public class Probe { public void run(Exception cause) { throw new " + typeName + "(" + test.arguments + "); } }"
				result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"Probe.cls": source}, "67.0")
				if result.HasErrors() != test.wantErrors {
					t.Fatalf("new %s(%s) errors = %v, diagnostics = %#v", typeName, test.arguments, result.HasErrors(), result.Diagnostics)
				}
				if test.wantErrors && !declarationDiagnosticMatching(result, "constructor") && !declarationDiagnosticMatching(result, "constructs non-instantiable") {
					t.Fatalf("new %s(%s) diagnostics = %#v, want constructor diagnostic", typeName, test.arguments, result.Diagnostics)
				}
			})
		}
	}
}

func TestNullPointerZeroConstructorRemainsAcceptedAtAPIVersions40And41(t *testing.T) {
	for _, apiVersion := range []string{"40.0", "41.0"} {
		t.Run(apiVersion, func(t *testing.T) {
			result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
				"Probe.cls": `public class Probe { public void run() { throw new NullPointerException(); } }`,
			}, apiVersion)
			if result.HasErrors() {
				t.Fatalf("API %s rejected throw new NullPointerException(): %#v", apiVersion, result.Diagnostics)
			}
		})
	}
}

func TestCustomExceptionSuppliedConstructorRemainsConstructable(t *testing.T) {
	result := analyzeDeclarationProject(t, map[string]string{
		"SuppliedException.cls": `public class SuppliedException extends Exception {
  public SuppliedException(String message) {}
}`,
		"Probe.cls": `public class Probe { public void run() { throw new SuppliedException('supplied'); } }`,
	})
	if result.HasErrors() {
		t.Fatalf("custom supplied exception constructor was rejected: %#v", result.Diagnostics)
	}
}
