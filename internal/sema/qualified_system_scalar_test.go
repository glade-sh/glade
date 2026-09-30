package sema

import (
	"github.com/glade-sh/glade/internal/typesys"
	"testing"
)

func TestQualifiedSystemBooleanReturnSupportsBitwiseAtAPI66(t *testing.T) {
	result := analyzeDeclarationProjectWithAPIVersion(t, map[string]string{
		"BooleanConsumer.cls": `public class BooleanConsumer {
   public static void check() {
    if (System.Test.isRunningTest() & true) {}
    Boolean result = System.Test.isRunningTest();
    if (!System.Test.isRunningTest()) {}
    Integer length = System.String.valueOf(1).length() + 1;
   }
  }`,
	}, "66.0")
	if result.HasErrors() {
		t.Fatalf("built-in return types rejected: %#v", result.Diagnostics)
	}
}

func TestQualifiedSystemReturnTypeCanonicalBuiltins(t *testing.T) {
	view := buildSemaTypeMemberState(typesys.Index{}, nil).view()
	for _, name := range []string{"Boolean", "Integer", "Long", "Decimal", "Double", "String", "Id", "Date", "Datetime", "Time", "Blob", "Object", "SObject", "List", "Set", "Map", "Iterable", "Iterator"} {
		for _, spelling := range []string{name, "System." + name} {
			if got := resolveNestedTypeName(view, "System.HttpResponse", spelling); got != name {
				t.Errorf("resolve %s = %s, want %s", spelling, got, name)
			}
		}
	}
	if got := resolveNestedTypeReference(view, "System.RestResponse", "System.Map<System.String,System.List<System.Boolean>>"); got != "Map<String,List<Boolean>>" {
		t.Errorf("nested built-ins = %s", got)
	}
	if got := resolveNestedTypeName(view, "System.HttpCalloutMock", "HttpResponse"); got != "System.HttpResponse" {
		t.Errorf("platform class identity = %s", got)
	}
}
