package apexlang

import (
	"strings"
	"testing"
)

func TestAnnotationCatalogRecognizesSupportedAndPreviewEntries(t *testing.T) {
	if spec, ok := LookupAnnotation("AuraEnabled"); !ok || spec.Preview {
		t.Fatalf("AuraEnabled = %#v, %v", spec, ok)
	}
	if spec, ok := LookupAnnotation("IntegrationTest"); !ok || !spec.Preview {
		t.Fatalf("IntegrationTest = %#v, %v", spec, ok)
	}
	if _, ok := LookupAnnotation("DoesNotExist"); ok {
		t.Fatal("unknown annotation unexpectedly recognized")
	}
	if _, ok := LookupAnnotation("webservice"); ok {
		t.Fatal("webservice is an Apex modifier, not an annotation")
	}
}

func TestAnnotationCatalogIncludesDocumentedModifierValueKinds(t *testing.T) {
	tests := []struct {
		annotation string
		argument   string
		kind       AnnotationArgumentKind
	}{
		{annotation: "IsTest", argument: "critical", kind: AnnotationBooleanArgument},
		{annotation: "IsTest", argument: "testFor", kind: AnnotationStringArgument},
		{annotation: "InvocableMethod", argument: "capabilityType", kind: AnnotationStringArgument},
		{annotation: "InvocableMethod", argument: "configurationEditor", kind: AnnotationStringArgument},
		{annotation: "InvocableMethod", argument: "iconName", kind: AnnotationStringArgument},
		{annotation: "InvocableVariable", argument: "defaultValue", kind: AnnotationStringArgument},
		{annotation: "InvocableVariable", argument: "placeholderText", kind: AnnotationStringArgument},
	}
	for _, test := range tests {
		t.Run(test.annotation+"/"+test.argument, func(t *testing.T) {
			spec, ok := LookupAnnotation(test.annotation)
			if !ok {
				t.Fatalf("annotation %q is missing", test.annotation)
			}
			if got := spec.Arguments[strings.ToLower(test.argument)]; got != test.kind {
				t.Fatalf("%s.%s kind = %q, want %q", test.annotation, test.argument, got, test.kind)
			}
		})
	}
}
