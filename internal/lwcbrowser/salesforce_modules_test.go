package lwcbrowser

import (
	"strings"
	"testing"
)

func TestParseApexWireToken(t *testing.T) {
	className, methodName, ok := ParseApexWireToken("ItemCtrl.getItems")
	if !ok || className != "ItemCtrl" || methodName != "getItems" {
		t.Fatalf("class=%q method=%q ok=%v", className, methodName, ok)
	}
}

func TestParseSchemaFieldToken(t *testing.T) {
	objectName, fieldName, ok := ParseSchemaFieldToken("Account.Name")
	if !ok || objectName != "Account" || fieldName != "Name" {
		t.Fatalf("object=%q field=%q ok=%v", objectName, fieldName, ok)
	}
}

func TestApexWireModuleJS(t *testing.T) {
	js := ApexWireModuleJS("ItemCtrl", "getItems")
	if !containsAll(js, "createApexWireAdapter", "ItemCtrl", "getItems") {
		t.Fatalf("js = %q", js)
	}
}

func containsAll(s string, parts ...string) bool {
	for _, part := range parts {
		if !strings.Contains(s, part) {
			return false
		}
	}
	return true
}
