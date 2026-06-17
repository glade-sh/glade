package lwcbrowser

import (
	"os"
	"os/exec"
	"path/filepath"
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

func TestParseSchemaObjectToken(t *testing.T) {
	objectName, ok := ParseSchemaObjectToken("Account")
	if !ok || objectName != "Account" {
		t.Fatalf("object=%q ok=%v", objectName, ok)
	}
}

func TestApexWireModuleJS(t *testing.T) {
	js := ApexWireModuleJS("ItemCtrl", "getItems")
	if !containsAll(js, "createApexWireAdapter", "ItemCtrl", "getItems", "export default") {
		t.Fatalf("js = %q", js)
	}
}

func TestSalesforceImportMapIncludesPhase8Shims(t *testing.T) {
	imports := SalesforceImportMap()
	for _, specifier := range []string{
		"@salesforce/community/basePath",
		"@salesforce/community/Id",
		"@salesforce/site/Id",
		"@salesforce/user/",
		"@salesforce/i18n/",
		"@salesforce/apex",
		"@salesforce/resourceUrl/",
		"@salesforce/contentAssetUrl/",
		"@salesforce/messageChannel/",
		"lightning/navigation",
		"lightning/platformShowToastEvent",
		"lightning/platformResourceLoader",
		"lightning/messageService",
		"lightning/actions",
		"lightning/empApi",
		"lightning/flowSupport",
		"lightning/refresh",
		"lightning/platformWorkspaceApi",
		"lightning/uiLayoutApi",
		"lightning/uiListApi",
		"lightning/uiObjectInfoApi",
		"lightning/uiRelatedListApi",
		"lightning/",
	} {
		if imports[specifier] == "" {
			t.Fatalf("missing import map entry for %s in %#v", specifier, imports)
		}
	}
}

func TestMessageChannelModuleJSExportsChannelToken(t *testing.T) {
	js := MessageChannelModuleJS("LwcProbe__c")
	if !containsAll(js, `name: "LwcProbe__c"`, `messageChannelName: "LwcProbe__c"`, "export default channel") {
		t.Fatalf("message channel js = %q", js)
	}
}

func TestPackagePhase1ServiceModulesExportLocalContracts(t *testing.T) {
	cases := map[string][]string{
		"actions":     {ActionsModuleJS(), "CloseActionScreenEvent", "closeactionscreen"},
		"empApi":      {EmpAPIModuleJS(), "subscribe", "unsubscribe", "isEmpEnabled"},
		"flowSupport": {FlowSupportModuleJS(), "FlowAttributeChangeEvent", "flownavigationnext", "flownavigationfinish"},
		"refresh":     {RefreshModuleJS(), "RefreshEvent", "registerRefreshHandler", "unregisterRefreshHandler"},
	}
	for name, parts := range cases {
		js := parts[0]
		if !containsAll(js, parts[1:]...) {
			t.Fatalf("%s module js = %q", name, js)
		}
	}
}

func TestSalesforceImportMapIncludesShellSLDSAndBaseModules(t *testing.T) {
	imports := SalesforceImportMap()
	want := map[string]string{
		"@glade/shell/app":          "/lightning/runtime/shell/app.js",
		"@glade/shell/router":       "/lightning/runtime/shell/router.js",
		"@glade/shell/contextPanel": "/lightning/runtime/shell/context-panel.js",
		"@glade/shell/diagnostics":  "/lightning/runtime/shell/diagnostics.js",
		"@glade/slds":               "/lightning/runtime/slds/slds-loader.js",
		"lightning/button":          "/lightning/shims/lightning/button.js",
		"lightning/buttonIcon":      "/lightning/shims/lightning/buttonIcon.js",
		"lightning/recordEditForm":  "/lightning/shims/lightning/recordEditForm.js",
		"lightning/messages":        "/lightning/shims/lightning/messages.js",
		"lightning/dualListbox":     "/lightning/shims/lightning/dualListbox.js",
		"lightning/treeGrid":        "/lightning/shims/lightning/treeGrid.js",
	}
	for specifier, path := range want {
		if imports[specifier] != path {
			t.Fatalf("import map %s = %q, want %q in %#v", specifier, imports[specifier], path, imports)
		}
	}
}

func TestLightningBaseComponentModuleJSExportsPracticalComponent(t *testing.T) {
	js := LightningBaseComponentModuleJS("combobox")
	if !containsAll(js, "LightningElement", "registerTemplate", "registerComponent", "lightning-combobox", "slds-combobox", "change", "export default") {
		t.Fatalf("js = %q", js)
	}
}

func TestUserModuleJS(t *testing.T) {
	if got := UserModuleJS("Id", "005000000000123"); !strings.Contains(got, `export default "005000000000123"`) {
		t.Fatalf("Id js = %q", got)
	}
	if got := UserModuleJS("isGuest", ""); !containsAll(got, `export default readGuest()`, "readCommunityContext") {
		t.Fatalf("isGuest js = %q", got)
	}
	if got := UserModuleJS("LocaleSidKey", ""); !strings.Contains(got, "Unsupported @salesforce/user property") {
		t.Fatalf("unsupported user js = %q", got)
	}
}

func TestCommunityAndSiteModuleJS(t *testing.T) {
	if got := CommunityModuleJS("basePath"); !containsAll(got, `export default readCommunityValue("basePath", "/s")`, "readCommunityValue") {
		t.Fatalf("community basePath js = %q", got)
	}
	if got := CommunityModuleJS("Id"); !containsAll(got, `export default readCommunityValue("networkId", "")`, "readCommunityValue") {
		t.Fatalf("community Id js = %q", got)
	}
	if got := SiteModuleJS("Id"); !containsAll(got, `export default readSiteId()`, "readSiteId") {
		t.Fatalf("site Id js = %q", got)
	}
	if got := CommunityModuleJS("Theme"); !strings.Contains(got, "Unsupported @salesforce/community property") {
		t.Fatalf("unsupported community js = %q", got)
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

func TestNavigationModuleJSSupportsCurrentPageReferenceAndURLs(t *testing.T) {
	js := NavigationModuleJS()
	if !containsAll(js,
		"NavigationMixin",
		"GenerateUrl",
		"Navigate",
		"CurrentPageReference",
		"generateUrl",
		"navigate",
		"CurrentPageReferenceAdapter",
		"standard__recordPage",
		"standard__objectPage",
		"standard__recordRelationshipPage",
		"standard__navItemPage",
		"standard__app",
		"standard__namedPage",
		"standard__component",
		"standard__quickAction",
		"standard__webPage",
		"comm__namedPage",
		"comm__loginPage",
		"comm__managedContentPage",
		"comm__recordPage",
		"comm__recordRelationshipPage",
		"GLADELWC040",
		"GLADELWC041",
		"GLADELWC042",
		"GLADELWC103",
	) {
		t.Fatalf("js = %q", js)
	}
	if strings.Contains(js, "window.location.assign") {
		t.Fatalf("navigation shim should delegate browser changes to the shell service: %q", js)
	}
}

func TestPlatformWorkspaceAPIModuleJSExportsLocalApproximation(t *testing.T) {
	js := PlatformWorkspaceAPIModuleJS()
	if !containsAll(js,
		"getFocusedTabInfo",
		"setTabLabel",
		"getAllTabInfo",
		"GLADELWC072",
		"glade-lwc-workbench",
		"activeRoute",
	) {
		t.Fatalf("js = %q", js)
	}
}

func TestUIRecordAPIModuleJSExportsRecordAndObjectInfoWires(t *testing.T) {
	js := UIRecordAPIModuleJS()
	if !containsAll(js,
		"createGetRecordWireAdapter",
		"lds-cache.mjs",
		"getRecord",
		"getRecords",
		"getRecordCreateDefaults",
		"getObjectInfo",
		"getObjectInfos",
		"getPicklistValues",
		"getPicklistValuesByRecordType",
		"getRelatedListRecords",
		"getListUi",
		"GLADELWC050",
		"notifyRecordUpdateAvailable",
		"refreshApex",
		"objectApiName: objectApiName",
		"createRecord",
		"updateRecord",
		"deleteRecord",
		"generateRecordInputForCreate",
		"generateRecordInputForUpdate",
		"createRecordInputFilteredByEditedFields",
		"getFieldValue",
		"getFieldDisplayValue",
	) {
		t.Fatalf("js = %q", js)
	}
}

func TestUIListAPIModuleJSExportsGetListUiDiagnostic(t *testing.T) {
	js := UIListAPIModuleJS()
	if !containsAll(js, "getListUi", "GLADELWC050", "getRelatedListRecords") {
		t.Fatalf("js = %q", js)
	}
}

func TestUIRecordAPIRecordInputHelpersFilterAndUnwrapFields(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	dir := t.TempDir()
	shimDir := filepath.Join(dir, "lightning", "shims", "core")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"type":"module"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	wireStub := `export function createFetchWireAdapter() { return class {}; }
export function createGetRecordWireAdapter() { return class {}; }
`
	if err := os.WriteFile(filepath.Join(shimDir, "wire-adapter.js"), []byte(wireStub), 0o644); err != nil {
		t.Fatal(err)
	}
	cacheStub := `export function getRecordNotifyChange() {}
export function notifyRecordUpdateAvailable() { return Promise.resolve(); }
export function refreshApex() { return Promise.resolve(); }
`
	if err := os.WriteFile(filepath.Join(shimDir, "lds-cache.mjs"), []byte(cacheStub), 0o644); err != nil {
		t.Fatal(err)
	}
	moduleJS := strings.ReplaceAll(UIRecordAPIModuleJS(), `"/lightning/shims/core/wire-adapter.js"`, `"./lightning/shims/core/wire-adapter.js"`)
	moduleJS = strings.ReplaceAll(moduleJS, `"/lightning/shims/core/lds-cache.mjs"`, `"./lightning/shims/core/lds-cache.mjs"`)
	if err := os.WriteFile(filepath.Join(dir, "uiRecordApi.mjs"), []byte(moduleJS), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `
import assert from "node:assert/strict";
import {
  createRecordInputFilteredByEditedFields,
  generateRecordInputForCreate,
  generateRecordInputForUpdate,
} from "./uiRecordApi.mjs";

const objectInfo = {
  fields: {
    Name: { createable: true, updateable: true },
    Secret__c: { createable: false, updateable: false },
    Owner: { createable: true, updateable: true },
  },
};
const record = {
  id: "001XX0000000001",
  apiName: "Account",
  fields: {
    Id: { value: "001XX0000000001" },
    Name: { value: "Acme" },
    Secret__c: { value: "hidden" },
    Owner: { value: { fields: { Name: { value: "Ada" } } } },
  },
};

assert.deepEqual(generateRecordInputForCreate(record, objectInfo), {
  apiName: "Account",
  fields: { Name: "Acme" },
});
assert.deepEqual(generateRecordInputForUpdate(record, objectInfo), {
  fields: { Name: "Acme", Id: "001XX0000000001" },
});
assert.deepEqual(createRecordInputFilteredByEditedFields(
  { fields: { Id: "001XX0000000001", Name: "Acme", Phone: "555" } },
  { fields: { Name: { value: "Acme" }, Phone: { value: "444" } } },
), {
  fields: { Id: "001XX0000000001", Phone: "555" },
});
`
	if err := os.WriteFile(filepath.Join(dir, "test.mjs"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "test.mjs")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node helper test failed: %v\n%s", err, output)
	}
}

func TestUILayoutAPIModuleJSMapsGetLayoutRequest(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	dir := t.TempDir()
	shimDir := filepath.Join(dir, "lightning", "shims", "core")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"type":"module"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	wireStub := `export const adapters = [];
export function createFetchWireAdapter(url, mapper) {
  adapters.push({ url, mapper });
  return class {};
}
`
	if err := os.WriteFile(filepath.Join(shimDir, "wire-adapter.js"), []byte(wireStub), 0o644); err != nil {
		t.Fatal(err)
	}
	moduleJS := strings.ReplaceAll(UILayoutAPIModuleJS(), `"/lightning/shims/core/wire-adapter.js"`, `"./lightning/shims/core/wire-adapter.js"`)
	if err := os.WriteFile(filepath.Join(dir, "uiLayoutApi.mjs"), []byte(moduleJS), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `
import assert from "node:assert/strict";
import { adapters } from "./lightning/shims/core/wire-adapter.js";
import { getLayout } from "./uiLayoutApi.mjs";
assert.equal(typeof getLayout, "function");
assert.equal(adapters.length, 1);
assert.equal(adapters[0].url, "/lightning/wire/getLayout");
assert.deepEqual(adapters[0].mapper({
  objectApiName: { objectApiName: "Account" },
  recordTypeId: "012000000000123",
  layoutType: "Full",
  mode: "Create",
  formFactor: "Small",
}), {
  objectApiName: "Account",
  recordTypeId: "012000000000123",
  layoutType: "Full",
  mode: "Create",
  formFactor: "Small",
});
`
	if err := os.WriteFile(filepath.Join(dir, "test.mjs"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "test.mjs")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node uiLayoutApi test failed: %v\n%s", err, output)
	}
}

func TestUIObjectInfoAPIModuleJSMapsObjectAndPicklistRequests(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	dir := t.TempDir()
	shimDir := filepath.Join(dir, "lightning", "shims", "core")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"type":"module"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	wireStub := `export const adapters = [];
export function createFetchWireAdapter(url, mapper) {
  adapters.push({ url, mapper });
  return class {};
}
`
	if err := os.WriteFile(filepath.Join(shimDir, "wire-adapter.js"), []byte(wireStub), 0o644); err != nil {
		t.Fatal(err)
	}
	moduleJS := strings.ReplaceAll(UIObjectInfoAPIModuleJS(), `"/lightning/shims/core/wire-adapter.js"`, `"./lightning/shims/core/wire-adapter.js"`)
	if err := os.WriteFile(filepath.Join(dir, "uiObjectInfoApi.mjs"), []byte(moduleJS), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `
import assert from "node:assert/strict";
import { adapters } from "./lightning/shims/core/wire-adapter.js";
import {
  getObjectInfo,
  getObjectInfos,
  getPicklistValues,
  getPicklistValuesByRecordType,
} from "./uiObjectInfoApi.mjs";
assert.equal(typeof getObjectInfo, "function");
assert.equal(typeof getObjectInfos, "function");
assert.equal(typeof getPicklistValues, "function");
assert.equal(typeof getPicklistValuesByRecordType, "function");
assert.deepEqual(adapters.map((adapter) => adapter.url), [
  "/lightning/wire/getObjectInfo",
  "/lightning/wire/getObjectInfos",
  "/lightning/wire/getPicklistValues",
  "/lightning/wire/getPicklistValuesByRecordType",
]);
assert.deepEqual(adapters[0].mapper({ objectApiName: { objectApiName: "Account" } }), { objectApiName: "Account" });
assert.deepEqual(adapters[1].mapper({ objectApiNames: [{ objectApiName: "Account" }, "Contact"] }), { objectApiNames: ["Account", "Contact"] });
assert.deepEqual(adapters[2].mapper({ fieldApiName: { objectApiName: "Account", fieldApiName: "Rating" }, recordTypeId: "012000000000123" }), {
  objectApiName: undefined,
  fieldApiName: "Account.Rating",
  recordTypeId: "012000000000123",
});
assert.deepEqual(adapters[3].mapper({ objectApiName: { objectApiName: "Account" }, recordTypeId: "012000000000123" }), {
  objectApiName: "Account",
  recordTypeId: "012000000000123",
});
`
	if err := os.WriteFile(filepath.Join(dir, "test.mjs"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "test.mjs")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node uiObjectInfoApi test failed: %v\n%s", err, output)
	}
}

func TestUIRelatedListAPIModuleJSMapsRecordRequests(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	dir := t.TempDir()
	shimDir := filepath.Join(dir, "lightning", "shims", "core")
	if err := os.MkdirAll(shimDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "package.json"), []byte(`{"type":"module"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	wireStub := `export const adapters = [];
export function createFetchWireAdapter(url, mapper) {
  adapters.push({ url, mapper });
  return class {};
}
`
	if err := os.WriteFile(filepath.Join(shimDir, "wire-adapter.js"), []byte(wireStub), 0o644); err != nil {
		t.Fatal(err)
	}
	moduleJS := strings.ReplaceAll(UIRelatedListAPIModuleJS(), `"/lightning/shims/core/wire-adapter.js"`, `"./lightning/shims/core/wire-adapter.js"`)
	if err := os.WriteFile(filepath.Join(dir, "uiRelatedListApi.mjs"), []byte(moduleJS), 0o644); err != nil {
		t.Fatal(err)
	}
	script := `
import assert from "node:assert/strict";
import { adapters } from "./lightning/shims/core/wire-adapter.js";
import { getRelatedListRecords } from "./uiRelatedListApi.mjs";
assert.equal(typeof getRelatedListRecords, "function");
assert.equal(adapters.length, 1);
assert.equal(adapters[0].url, "/lightning/wire/getRelatedListRecords");
assert.deepEqual(adapters[0].mapper({
  parentRecordId: "001000000000001AAA",
  relatedListId: "Contacts",
  fields: [{ objectApiName: "Contact", fieldApiName: "LastName" }],
  optionalFields: ["Contact.Email"],
  sortBy: ["Contact.LastName"],
  pageSize: 5,
  pageToken: "2"
}), {
  parentRecordId: "001000000000001AAA",
  relatedListId: "Contacts",
  fields: ["Contact.LastName"],
  optionalFields: ["Contact.Email"],
  sortBy: ["Contact.LastName"],
  pageSize: 5,
  pageToken: "2"
});
`
	if err := os.WriteFile(filepath.Join(dir, "test.mjs"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "test.mjs")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("node uiRelatedListApi test failed: %v\n%s", err, output)
	}
}

func TestLightningServiceModulesExportUsableStubs(t *testing.T) {
	if !containsAll(ShowToastEventModuleJS(), "ShowToastEvent", "lightning__showtoast", "recordToast") {
		t.Fatalf("toast js = %q", ShowToastEventModuleJS())
	}
	if !containsAll(PlatformResourceLoaderModuleJS(), "loadScript", "loadStyle", "Promise.resolve") {
		t.Fatalf("resource loader js = %q", PlatformResourceLoaderModuleJS())
	}
	if !containsAll(MessageServiceModuleJS(), "publish", "subscribe", "unsubscribe", "MessageContext", "releaseMessageContext") {
		t.Fatalf("message service js = %q", MessageServiceModuleJS())
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
