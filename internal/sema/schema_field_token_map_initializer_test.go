package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

// C002's String-key/String-value initializer is rejected with this exact
// problem text at API 62/67/68 on anonymous and @IsTest routes. The transport
// helper call is omitted here; full row bodies remain in the conformance export.
func TestSchemaFieldTokenMapInitializerRejectsStringValue(t *testing.T) {
	const source = `Map<String,Schema.SObjectField> fields=new Map<String,Schema.SObjectField>{'slot__c'=>'Name'};`
	const want = "Invalid value type String for Schema.SObjectField"
	for _, api := range []string{"62.0", "67.0", "68.0"} {
		for _, route := range []string{"anonymous", "isTest"} {
			t.Run(api+"/"+route, func(t *testing.T) {
				var result Result
				if route == "anonymous" {
					result = AnalyzeAnonymous(typesys.Index{}, source, api)
				} else {
					result = analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
						"FieldTokenMapValueProbe.cls": "@IsTest private class FieldTokenMapValueProbe { @IsTest static void observed(){" + source + "} }",
					}, api)
				}
				var errors []diagnostic.Diagnostic
				for _, d := range result.Diagnostics {
					if d.Severity == diagnostic.Error {
						errors = append(errors, d)
					}
				}
				if len(errors) != 1 {
					t.Fatalf("want one native value rejection, got %#v", result.Diagnostics)
				}
				if d := errors[0]; d.Code != "GLADESEMA025" || d.Message != want || d.NativeMessage != want {
					t.Fatalf("want native map value rejection %q, got %#v", want, d)
				}
			})
		}
	}
}
