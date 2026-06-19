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
		"@salesforce/site/activeLanguages",
		"@salesforce/user/",
		"@salesforce/userPermission/",
		"@salesforce/apexContinuation",
		"@salesforce/apexContinuation/",
		"@salesforce/i18n/",
		"@salesforce/apex",
		"@salesforce/resourceUrl/",
		"@salesforce/contentAssetUrl/",
		"@salesforce/customPermission/",
		"@salesforce/messageChannel/",
		"lightning/navigation",
		"lightning/pageReferenceUtils",
		"lightning/platformShowToastEvent",
		"lightning/platformResourceLoader",
		"lightning/messageService",
		"lightning/actions",
		"lightning/alert",
		"lightning/confirm",
		"lightning/configProvider",
		"lightning/empApi",
		"lightning/flowSupport",
		"lightning/prompt",
		"lightning/refresh",
		"lightning/showToastEvent",
		"lightning/toast",
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
		"actions":            {ActionsModuleJS(), "CloseActionScreenEvent", "closeactionscreen"},
		"alert":              {AlertModuleJS(), "LightningAlert", "static open", "gladealert"},
		"confirm":            {ConfirmModuleJS(), "LightningConfirm", "Promise.resolve(true)"},
		"configProvider":     {ConfigProviderModuleJS(), "getPathPrefix", "getToken", "getIconSvgTemplates", "getLocalizationService", "getOneConfig"},
		"customPermission":   {CustomPermissionModuleJS("LocalPermission"), "permissionName", "LocalPermission", "export default true"},
		"empApi":             {EmpAPIModuleJS(), "subscribe", "unsubscribe", "isEmpEnabled"},
		"flowSupport":        {FlowSupportModuleJS(), "FlowAttributeChangeEvent", "flownavigationnext", "flownavigationfinish"},
		"pageReferenceUtils": {PageReferenceUtilsModuleJS(), "encodeDefaultFieldValues", "decodeDefaultFieldValues"},
		"prompt":             {PromptModuleJS(), "LightningPrompt", "static open", "gladeprompt"},
		"refresh":            {RefreshModuleJS(), "RefreshEvent", "registerRefreshHandler", "unregisterRefreshHandler"},
		"showToastEvent":     {ShowToastEventModuleJS(), "SHOW_TOAST_EVENT_NAME", "ShowToastEvent", "lightning__showtoast"},
		"toast":              {ToastModuleJS(), "LightningToast", "static show", "lightning__showtoast"},
	}
	for name, parts := range cases {
		js := parts[0]
		if !containsAll(js, parts[1:]...) {
			t.Fatalf("%s module js = %q", name, js)
		}
	}
}

func TestNPMPackageExposedUtilityModulesResolveLocally(t *testing.T) {
	for _, name := range []string{
		"ariaObserver",
		"context",
		"datatableKeyboardMixins",
		"f6Controller",
		"fileDownload",
		"i18nCldrOptions",
		"i18nService",
		"iconSvgTemplates",
		"iconSvgTemplatesAction",
		"iconSvgTemplatesActionRtl",
		"iconSvgTemplatesCustom",
		"iconSvgTemplatesCustomRtl",
		"iconSvgTemplatesDoctype",
		"iconSvgTemplatesDoctypeRtl",
		"iconSvgTemplatesRtl",
		"iconSvgTemplatesStandard",
		"iconSvgTemplatesStandardRtl",
		"iconSvgTemplatesUtility",
		"iconSvgTemplatesUtilityRtl",
		"iconUtils",
		"internalLocalizationService",
		"mediaUtils",
		"messageDispatcher",
		"overlayManager",
		"purifyLib",
		"routingService",
		"utils",
	} {
		js, ok := LightningUtilityModuleJS(name)
		if !ok {
			t.Fatalf("%s should resolve as a local lightning utility shim", name)
		}
		if !strings.Contains(js, "export") {
			t.Fatalf("%s utility shim has no exports: %q", name, js)
		}
	}
}

func TestConfigProviderModuleSupportsBaseComponentRecipesContracts(t *testing.T) {
	js := ConfigProviderModuleJS()
	if !containsAll(js,
		"export default configProviderService",
		"export function getLocalizationService",
		"export function getOneConfig",
		"isBefore(date1, date2, unit)",
		"isAfter(date1, date2, unit)",
		"formatDateTimeUTC(date)",
		"formatDate(dateString, format, locale)",
		"formatDateUTC(dateString, format, locale)",
		"formatTime(timeString, format)",
		"parseDateTimeUTC(dateTimeString)",
		"parseDateTimeISO8601(dateTimeString)",
		"parseDateTime(dateTimeString, format, strictMode)",
		"UTCToWallTime(date, timezone, callback)",
		"WallTimeToUTC(date, timezone, callback)",
		"translateToLocalizedDigits(input)",
		"translateFromLocalizedDigits(input)",
		"getNumberFormat(format)",
		"duration(value, unit)",
		"displayDuration(value, withSuffix)",
		"densitySetting",
	) {
		t.Fatalf("config provider js = %q", js)
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

func TestSalesforceImportMapIncludesPhase2NativeAPIShims(t *testing.T) {
	imports := SalesforceImportMap()
	want := map[string]string{
		"lightning/uiAppsApi":             "/lightning/runtime/lightning/uiAppsApi.js",
		"lightning/uiListsApi":            "/lightning/runtime/lightning/uiListsApi.js",
		"lightning/graphql":               "/lightning/runtime/lightning/graphql.js",
		"lightning/uiGraphQLApi":          "/lightning/runtime/lightning/uiGraphQLApi.js",
		"lightning/platformUtilityBarApi": "/lightning/runtime/lightning/platformUtilityBarApi.js",
		"lightning/uiLearningPlatformApi": "/lightning/runtime/lightning/uiLearningPlatformApi.js",
		"experience/blockBuilderApi":      "/lightning/runtime/experience/blockBuilderApi.js",
		"experience/cmsDeliveryApi":       "/lightning/runtime/experience/cmsDeliveryApi.js",
		"experience/cmsEditorApi":         "/lightning/runtime/experience/cmsEditorApi.js",
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

func TestLightningSourceBackedComponentModuleJSWrapsCompiledRuntimeAsset(t *testing.T) {
	js, ok := LightningSourceBackedComponentModuleJS("badge")
	if !ok {
		t.Fatalf("badge should be source backed")
	}
	if !containsAll(js, `export { default }`, `/lightning/runtime/lightning/source/badge/badge.js`) {
		t.Fatalf("source backed badge js = %q", js)
	}
	js, ok = LightningSourceBackedComponentModuleJS("recordPicker")
	if !ok {
		t.Fatalf("recordPicker should be source backed")
	}
	if !containsAll(js, `export { default }`, `/lightning/runtime/lightning/recordPicker.js`) {
		t.Fatalf("source backed recordPicker js = %q", js)
	}
	if _, ok := LightningSourceBackedComponentModuleJS("combobox"); ok {
		t.Fatalf("combobox should still use generated shim until its source graph is allowlisted")
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
	if got := SiteModuleJS("activeLanguages"); !containsAll(got, `export default readActiveLanguages()`, "readActiveLanguages") {
		t.Fatalf("site activeLanguages js = %q", got)
	}
	if got := CommunityModuleJS("Theme"); !strings.Contains(got, "Unsupported @salesforce/community property") {
		t.Fatalf("unsupported community js = %q", got)
	}
}

func TestUserPermissionModuleJS(t *testing.T) {
	js := UserPermissionModuleJS("ViewSetup")
	if !containsAll(js, `permissionName = "ViewSetup"`, `export default readUserPermission(permissionName)`, "readUserPermission") {
		t.Fatalf("user permission js = %q", js)
	}
}

func TestApexContinuationModuleJS(t *testing.T) {
	js := ApexContinuationModuleJS()
	if !containsAll(js, "createContinuation", "invokeContinuation", "Promise.resolve", "supported-local-simulated") {
		t.Fatalf("apex continuation js = %q", js)
	}
	methodJS := ApexContinuationMethodModuleJS("GladeLwcOracleController.ping")
	if !containsAll(methodJS, `methodName = "GladeLwcOracleController.ping"`, "invokeContinuation", "export default function") {
		t.Fatalf("apex continuation method js = %q", methodJS)
	}
}

func TestSimulatedNativeAPIModuleJS(t *testing.T) {
	js, ok := SimulatedNativeAPIModuleJS("analyticsWaveApi")
	if !ok {
		t.Fatalf("analyticsWaveApi should be a simulated native API module")
	}
	if !containsAll(js, `moduleName = "analyticsWaveApi"`, "partial-local-simulated", "export default") {
		t.Fatalf("analyticsWaveApi js = %q", js)
	}
	if _, ok := SimulatedNativeAPIModuleJS("notARealApi"); ok {
		t.Fatalf("unknown native API module should not be simulated")
	}
}

func TestI18nModuleJS(t *testing.T) {
	cases := map[string]string{
		"dateTime.mediumDateFormat": `export default "MMM d, yyyy"`,
		"dateTime.mediumTimeFormat": `export default "h:mm:ss a"`,
		"dir":                       `export default "ltr"`,
		"lang":                      `export default "en-US"`,
		"locale":                    `export default "en-US"`,
		"number.currencyFormat":     `export default "¤#,##0.00;(¤#,##0.00)"`,
		"number.numberFormat":       `export default "#,##0.###"`,
		"number.percentFormat":      `export default "#,##0%"`,
		"timeZone":                  `export default "UTC"`,
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
		"closeTab",
		"disableTabClose",
		"focusTab",
		"getFocusedTabInfo",
		"setTabLabel",
		"setTabIcon",
		"getAllTabInfo",
		"getTabInfo",
		"openSubtab",
		"openTab",
		"refreshTab",
		"setTabHighlighted",
		"IsConsoleNavigation",
		"EnclosingTabId",
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
		"getRecordUi",
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
		"objectApiName: apiName",
		"createRecord",
		"updateRecord",
		"deleteRecord",
		"__gladeRecordPickerSearch",
		"/lightning/wire/recordPickerSearch",
		"generateRecordInputForCreate",
		"generateRecordInputForUpdate",
		"createRecordInputFilteredByEditedFields",
		"getFieldValue",
		"getFieldDisplayValue",
		"/lightning/wire/getRecordUi",
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
assert.equal(adapters[0].mapper({}), null);
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
assert.equal(adapters[0].mapper({}), null);
assert.deepEqual(adapters[1].mapper({ objectApiNames: [{ objectApiName: "Account" }, "Contact"] }), { objectApiNames: ["Account", "Contact"] });
assert.equal(adapters[1].mapper({ objectApiNames: [] }), null);
assert.deepEqual(adapters[2].mapper({ fieldApiName: { objectApiName: "Account", fieldApiName: "Rating" }, recordTypeId: "012000000000123" }), {
  fieldApiName: "Account.Rating",
  recordTypeId: "012000000000123",
});
assert.equal(adapters[2].mapper({ fieldApiName: "Rating", recordTypeId: "012000000000123" }), null);
assert.deepEqual(adapters[3].mapper({ objectApiName: { objectApiName: "Account" }, recordTypeId: "012000000000123" }), {
  objectApiName: "Account",
  recordTypeId: "012000000000123",
});
assert.equal(adapters[3].mapper({ objectApiName: "Account" }), null);
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
assert.equal(adapters[0].mapper({ relatedListId: "Contacts" }), null);
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
