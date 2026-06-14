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

func TestSalesforceImportMapIncludesPhase8Shims(t *testing.T) {
	imports := SalesforceImportMap()
	for _, specifier := range []string{
		"@salesforce/user/",
		"@salesforce/i18n/",
		"@salesforce/resourceUrl/",
		"@salesforce/contentAssetUrl/",
		"lightning/navigation",
	} {
		if imports[specifier] == "" {
			t.Fatalf("missing import map entry for %s in %#v", specifier, imports)
		}
	}
}

func TestUserModuleJS(t *testing.T) {
	if got := UserModuleJS("Id", "005000000000123"); !strings.Contains(got, `export default "005000000000123"`) {
		t.Fatalf("Id js = %q", got)
	}
	if got := UserModuleJS("isGuest", ""); !strings.Contains(got, `export default false`) {
		t.Fatalf("isGuest js = %q", got)
	}
	if got := UserModuleJS("LocaleSidKey", ""); !strings.Contains(got, "Unsupported @salesforce/user property") {
		t.Fatalf("unsupported user js = %q", got)
	}
}

func TestI18nModuleJS(t *testing.T) {
	cases := map[string]string{
		"lang":     `export default "en-US"`,
		"locale":   `export default "en_US"`,
		"timeZone": `export default "UTC"`,
	}
	for property, want := range cases {
		if got := I18nModuleJS(property); !strings.Contains(got, want) {
			t.Fatalf("%s js = %q, want %q", property, got, want)
		}
	}
}

func TestNavigationModuleJSReportsUnsupportedFeature(t *testing.T) {
	js := NavigationModuleJS()
	if !containsAll(js, "NavigationMixin", "GenerateUrl", "Navigate", "lightning/navigation is not implemented") {
		t.Fatalf("js = %q", js)
	}
}

func TestUIRecordAPIModuleJSExportsRecordAndObjectInfoWires(t *testing.T) {
	js := UIRecordAPIModuleJS()
	if !containsAll(js, "createGetRecordWireAdapter", "getRecord", "getObjectInfo") {
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
