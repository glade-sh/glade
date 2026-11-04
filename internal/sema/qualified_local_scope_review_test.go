package sema

import (
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

// Generic arguments and array elements retain the v0.3.3 analyzer behavior.
// These are local preservation controls, not Salesforce acceptance claims.
func TestQualifiedLocalGuardPreservesCompoundTypeAnalysis(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		for _, tc := range []struct{ name, source string }{
			{"list_argument", "List<System.String> values;"},
			{"map_argument", "Map<String,System.Integer> values;"},
			{"nested_argument", "List<List<System.String>> values;"},
			{"string_array", "System.String[] values;"},
			{"integer_array", "System.Integer[] values;"},
			{"qualified_list_array", "System.List<String>[] values;"},
		} {
			for _, route := range []string{"anonymous", "istest"} {
				t.Run(api+"/"+route+"/"+tc.name, func(t *testing.T) {
					var result Result
					if route == "anonymous" {
						result = AnalyzeAnonymous(typesys.Index{}, tc.source, api)
					} else {
						source := "@IsTest private class QualifiedLocalScopeTest { @IsTest static void observe() { " + tc.source + " } }"
						result = analyzeDeclarationProjectWithAPIVersion(t, map[string]string{"QualifiedLocalScopeTest.cls": source}, api)
					}
					found := false
					for _, d := range result.Diagnostics {
						if d.Code == "GLADESEMA006" && strings.HasPrefix(d.NativeMessage, "Invalid type: System.") {
							found = true
						}
					}
					if !found {
						t.Fatalf("shipping analyzer rejects compound element; got %+v", result.Diagnostics)
					}
				})
			}
		}
	}
}
