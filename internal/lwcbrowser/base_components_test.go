package lwcbrowser

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/gladehome"
)

func assertSupportedBaseComponentJS(t *testing.T, name, js string) {
	t.Helper()
	if containsAll(js, "registerComponent", "export default", "createBaseComponent") {
		return
	}
	if containsAll(js, `export { default }`, `/lightning/runtime/lightning/`) {
		return
	}
	t.Fatalf("%s module js = %q", name, js)
}

func baseComponentContractJS(t *testing.T, name string) string {
	t.Helper()
	js := LightningBaseComponentModuleJS(name)
	switch normalizeLightningBaseComponentName(name) {
	case "buttonicon":
		return js + "\n" + readRuntimeLightningFile(t, "buttonIcon.mjs")
	case "progressring":
		return js + "\n" + readRuntimeLightningFile(t, "progressRing.mjs")
	}
	component, ok := sourceBackedLightningComponentName(name)
	if !ok {
		return js
	}
	var parts []string
	if component == "recordPicker" {
		parts = append(parts, readRuntimeLightningFile(t, "recordPicker.mjs"))
	} else {
		parts = append(parts, readRuntimeLightningFile(t, filepath.Join("source", component, component+".js")))
		formJS := readRuntimeLightningFile(t, "lds-form.mjs")
		parts = append(parts, formJS)
		if strings.Contains(formJS, `from "lightning/uiRecordApi"`) {
			// The form delegates record reads and mutations to the product LDS module.
			parts = append(parts, UIRecordAPIModuleJS())
		}
	}
	return js + "\n" + strings.Join(parts, "\n")
}

func readRuntimeLightningFile(t *testing.T, rel string) string {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("..", "..", "lwcruntime", "src", "lightning", rel))
	if err != nil {
		t.Fatalf("read runtime lightning file %s: %v", rel, err)
	}
	return string(data)
}

func TestUnsupportedBaseComponentModuleJSSerializesDiagnosticPayload(t *testing.T) {
	def := baseComponentDefinition{
		Name: "module\"\\\n</script>\u2028\u2029",
		Tag:  "lightning-tag\"\\\n</script>\u2028\u2029",
	}
	js := unsupportedBaseComponentModuleJS(def)

	const prefix = "const payload = "
	start := strings.Index(js, prefix)
	if start < 0 {
		t.Fatalf("generated module missing payload declaration: %q", js)
	}
	start += len(prefix)
	end := strings.Index(js[start:], ";\nreportDiagnostic")
	if end < 0 {
		t.Fatalf("generated module missing payload terminator: %q", js)
	}
	var payload struct {
		Message string `json:"message"`
		Module  string `json:"module"`
		TagName string `json:"tagName"`
	}
	if err := json.Unmarshal([]byte(js[start:start+end]), &payload); err != nil {
		t.Fatalf("unmarshal diagnostic payload: %v", err)
	}
	if want := "GLADELWC060 base component unsupported: " + def.Tag; payload.Message != want {
		t.Fatalf("payload message = %q, want %q", payload.Message, want)
	}
	if payload.Module != def.Name {
		t.Fatalf("payload module = %q, want %q", payload.Module, def.Name)
	}
	if payload.TagName != def.Tag {
		t.Fatalf("payload tagName = %q, want %q", payload.TagName, def.Tag)
	}
	if strings.Contains(js, "</script>") {
		t.Fatalf("generated module contains raw closing script tag: %q", js)
	}
}

func TestLightningBaseComponentSupportTiers(t *testing.T) {
	for _, name := range []string{
		"button",
		"buttonIcon",
		"card",
		"input",
		"textarea",
		"combobox",
		"layout",
		"layoutItem",
		"tabset",
		"tab",
		"spinner",
		"icon",
		"datatable",
		"recordForm",
		"recordViewForm",
		"recordEditForm",
		"outputField",
		"inputField",
		"messages",
		"modal",
	} {
		if !IsLightningBaseComponentModule(name) {
			t.Fatalf("expected %s to be recognized as a lightning base component", name)
		}
		js := LightningBaseComponentModuleJS(name)
		assertSupportedBaseComponentJS(t, name, js)
		if strings.Contains(js, "GLADELWC060") {
			t.Fatalf("%s should be supported, got diagnostic js = %q", name, js)
		}
	}
}

func TestPackagePhase1BaseComponentsAreSupported(t *testing.T) {
	for _, name := range []string{
		"accordion",
		"accordionSection",
		"avatar",
		"badge",
		"buttonGroup",
		"buttonIconStateful",
		"buttonMenu",
		"buttonStateful",
		"checkboxGroup",
		"fileUpload",
		"flow",
		"formattedAddress",
		"formattedDateTime",
		"formattedNumber",
		"formattedPhone",
		"formattedRichText",
		"formattedText",
		"formattedTime",
		"formattedUrl",
		"helptext",
		"inputAddress",
		"menuItem",
		"menuSubheader",
		"pill",
		"pillContainer",
		"progressIndicator",
		"progressStep",
		"quickActionPanel",
		"radioGroup",
		"recordPicker",
		"tree",
		"verticalNavigation",
		"verticalNavigationItem",
		"verticalNavigationSection",
	} {
		js := LightningBaseComponentModuleJS(name)
		assertSupportedBaseComponentJS(t, name, js)
		if strings.Contains(js, "GLADELWC060") {
			t.Fatalf("%s should be supported for package phase 1, got diagnostic js = %q", name, js)
		}
	}
}

func TestPhase3BaseComponentsAreSupported(t *testing.T) {
	for _, name := range []string{
		"breadcrumb",
		"breadcrumbs",
		"carousel",
		"carouselImage",
		"dualListbox",
		"formattedEmail",
		"inputRichText",
		"map",
		"menuDivider",
		"progressBar",
		"progressRing",
		"select",
		"slider",
		"tile",
		"treeGrid",
	} {
		js := LightningBaseComponentModuleJS(name)
		assertSupportedBaseComponentJS(t, name, js)
		if strings.Contains(js, "GLADELWC060") {
			t.Fatalf("%s should be supported for phase 3, got diagnostic js = %q", name, js)
		}
	}
}

func TestNPMPackageExposedBaseComponentsAreSupportedLocally(t *testing.T) {
	for _, name := range []string{
		"alert",
		"barcodeScanner",
		"baseFormattedText",
		"dialog",
		"dynamicIcon",
		"focusTrap",
		"formattedLocation",
		"formattedLookup",
		"formattedName",
		"groupedCombobox",
		"inputLocation",
		"inputName",
		"lookupAddress",
		"modalBody",
		"modalFooter",
		"modalHeader",
		"multiColumnSortingModal",
		"overlay",
		"picklist",
		"popup",
		"primitiveFigure",
		"prompt",
		"relativeDateTime",
		"stackedTab",
		"stackedTabset",
		"toast",
		"toastContainer",
		"verticalNavigationItemBadge",
		"verticalNavigationItemIcon",
		"verticalNavigationOverflow",
	} {
		if !IsLightningBaseComponentModule(name) {
			t.Fatalf("expected %s to be recognized as a lightning base component", name)
		}
		js := LightningBaseComponentModuleJS(name)
		assertSupportedBaseComponentJS(t, name, js)
		if strings.Contains(js, "GLADELWC060") {
			t.Fatalf("%s should be supported from the lightning-base-components expose list, got diagnostic js = %q", name, js)
		}
	}
}

func TestBaseComponentModuleJSUsesCanonicalKebabTags(t *testing.T) {
	cases := map[string]string{
		"buttonIcon":     "lightning-button-icon",
		"layoutItem":     "lightning-layout-item",
		"recordViewForm": "lightning-record-view-form",
		"recordEditForm": "lightning-record-edit-form",
		"outputField":    "lightning-output-field",
		"inputField":     "lightning-input-field",
		"modal":          "lightning-modal",
		"dualListbox":    "lightning-dual-listbox",
		"inputRichText":  "lightning-input-rich-text",
		"inputLocation":  "lightning-input-location",
		"inputName":      "lightning-input-name",
		"progressRing":   "lightning-progress-ring",
		"treeGrid":       "lightning-tree-grid",
	}
	for name, want := range cases {
		if got := baseComponentContractJS(t, name); !strings.Contains(got, want) {
			t.Fatalf("%s module missing %s:\n%s", name, want, got)
		}
	}
}

func TestBaseComponentPublicPropsIncludeFieldName(t *testing.T) {
	js := baseComponentContractJS(t, "outputField")
	if !containsAll(js, `"fieldName"`, "$cmp.fieldName") {
		t.Fatalf("outputField module missing fieldName support:\n%s", js)
	}
}

func TestBaseComponentPublicMethodsExposeFormAndValidityContracts(t *testing.T) {
	js := baseComponentContractJS(t, "inputField")
	if !containsAll(js,
		"publicMethods: COMMON_METHODS",
		`"setErrors"`,
		`"getErrors"`,
		`"wireRecordUi"`,
		`"getWiredData"`,
		`"wirePicklistValues"`,
		`"getWiredPicklistValues"`,
		`"setValue"`,
		`"clean"`,
		`"reset"`,
		`"setCustomValidity"`,
		`"checkValidity"`,
		`"reportValidity"`,
		`"focus"`,
		`"blur"`,
	) {
		t.Fatalf("inputField module missing public method contracts:\n%s", js)
	}
}

func TestDatatableModuleDispatchesRowAction(t *testing.T) {
	js := baseComponentContractJS(t, "datatable")
	if !containsAll(js, "handleRowAction", `"rowaction"`, "typeAttributes", "rowActions") {
		t.Fatalf("datatable module missing row action support:\n%s", js)
	}
}

func TestPriorityBaseComponentsUseSourceBackedRuntimeModules(t *testing.T) {
	cases := map[string]string{
		"datatable":      "/lightning/runtime/lightning/source/datatable/datatable.js",
		"inputField":     "/lightning/runtime/lightning/source/inputField/inputField.js",
		"outputField":    "/lightning/runtime/lightning/source/outputField/outputField.js",
		"messages":       "/lightning/runtime/lightning/source/messages/messages.js",
		"recordForm":     "/lightning/runtime/lightning/source/recordForm/recordForm.js",
		"recordEditForm": "/lightning/runtime/lightning/source/recordEditForm/recordEditForm.js",
		"recordViewForm": "/lightning/runtime/lightning/source/recordViewForm/recordViewForm.js",
	}
	for name, want := range cases {
		js, ok := LightningSourceBackedComponentModuleJS(name)
		if !ok {
			t.Fatalf("%s should be source backed", name)
		}
		if !strings.Contains(js, want) {
			t.Fatalf("%s source module = %q, want %q", name, js, want)
		}
	}
}

func TestPriorityBaseComponentSourceFilesAreNotShimReexports(t *testing.T) {
	for _, component := range []string{
		"datatable",
		"inputField",
		"messages",
		"outputField",
		"recordForm",
		"recordEditForm",
		"recordViewForm",
	} {
		path := filepath.Join("..", "..", "lwcruntime", "src", "lightning", "source", component, component+".js")
		source, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s source file: %v", component, err)
		}
		js := string(source)
		if strings.Contains(js, "export { default } from \"../../") || strings.Contains(js, "export { default } from '../../") {
			t.Fatalf("%s source file should not re-export a local generated shim:\n%s", component, js)
		}
		if !containsAll(js, "create", "export default") {
			t.Fatalf("%s source file should define a source-backed component:\n%s", component, js)
		}
	}
}

func TestRecordFormModuleUsesLocalLDSEndpoints(t *testing.T) {
	js := baseComponentContractJS(t, "recordEditForm")
	if !containsAll(js,
		"loadRecord",
		`"/lightning/wire/getRecord"`,
		`"/lightning/wire/updateRecord"`,
		`"success"`,
		`"data-field-name"`,
		"readRecordDisplayValue",
	) {
		t.Fatalf("record form module missing LDS-backed rendering and submit support:\n%s", js)
	}
}

func TestTabModuleDispatchesActiveEvent(t *testing.T) {
	js := readRuntimeLightningFile(t, "tab.mjs")
	if !containsAll(js, "handleActive", `"active"`, "this.value", "this.label") {
		t.Fatalf("tab module missing active event support:\n%s", js)
	}
}

func TestBaseComponentModuleReportsUnsupportedAttributes(t *testing.T) {
	js := baseComponentContractJS(t, "datatable")
	if !containsAll(js, "GLADELWC061", "unsupportedAttrs", "getAttributeNames") {
		t.Fatalf("base component module missing unsupported attribute diagnostics:\n%s", js)
	}
}

func TestButtonModuleLetsNativeClickBubbleOnce(t *testing.T) {
	js := LightningBaseComponentModuleJS("button")
	if strings.Contains(js, "handleClick") || strings.Contains(js, `new CustomEvent("click"`) {
		t.Fatalf("button module should not redispatch native click events:\n%s", js)
	}
}

func TestBaseComponentBooleanDisabledUsesProperty(t *testing.T) {
	js := LightningBaseComponentModuleJS("button")
	if !containsAll(js, "props:", "disabled: Boolean($cmp.disabled)") {
		t.Fatalf("button module should set disabled as a boolean property:\n%s", js)
	}
	if strings.Contains(js, "attrs: { type: $cmp.type || \"button\", disabled: $cmp.disabled }") {
		t.Fatalf("button module should not set disabled as a string attribute:\n%s", js)
	}
}

func TestBaseComponentSourceReferenceContracts(t *testing.T) {
	cases := map[string][]string{
		"button": {
			"buttonClassMap($cmp.variant)",
			"normalizedButtonType($cmp.type)",
			"slds-button_brand",
			"name: $cmp.name || undefined",
			"value: $cmp.value == null ? undefined : String($cmp.value)",
		},
		"buttonIcon": {
			"buttonIconClassMap($cmp.variant, $cmp.size)",
			"slds-button_icon-border-filled",
			`slds-button_icon-small`,
			"value: $cmp.value == null ? undefined : String($cmp.value)",
		},
		"card": {
			"cardClassMap($cmp.variant)",
			"slds-card_narrow",
			`api_slot("actions"`,
			`api_slot("footer"`,
			"slds-no-flex",
		},
		"formattedNumber": {
			"formatNumberValue($cmp)",
			"percent-fixed",
			"currencyDisplayAs",
			"maximumSignificantDigits",
		},
		"layout": {
			"layoutClassMap($cmp)",
			"slds-grid_align-spread",
			"slds-grid_vertical-align-center",
		},
		"layoutItem": {
			"layoutItemClassMap($cmp)",
			"slds-size_",
			"slds-small-size_",
			"slds-p-around_",
		},
	}
	for name, parts := range cases {
		js := LightningBaseComponentModuleJS(name)
		if name == "buttonIcon" {
			js = baseComponentContractJS(t, name)
		}
		if !containsAll(js, parts...) {
			t.Fatalf("%s module missing source-reference contract parts %v:\n%s", name, parts, js)
		}
	}
}

func TestDatatableModuleKeepsActionColumnIndex(t *testing.T) {
	js := baseComponentContractJS(t, "datatable")
	if !containsAll(js, "columnIndex", "dataset.columnIndex", `"data-column-index"`) {
		t.Fatalf("datatable module should preserve action column index:\n%s", js)
	}
}

func TestModalModuleProvidesOpenStatic(t *testing.T) {
	js := LightningBaseComponentModuleJS("modal")
	if !containsAll(js, "static open", "/lightning/runtime/shell/modal-overlay.js", "openModal(this, options)", "close(result)", "closeModal(this, result)") {
		t.Fatalf("modal module missing open/close lifetime:\n%s", js)
	}
}

func TestPhase3BaseComponentsDispatchLocalEvents(t *testing.T) {
	cases := map[string][]string{
		"checkboxGroup": {"handleOptionGroupChange", `"change"`, "querySelectorAll"},
		"dualListbox":   {"handleDualListboxMove", `"change"`, `data-list`},
		"inputRichText": {"handleRichTextChange", `"change"`, "innerHTML"},
		"select":        {"handleChange", `"change"`, "slds-select"},
		"slider":        {"handleChange", `type: "range"`, "slds-slider"},
	}
	for name, parts := range cases {
		js := LightningBaseComponentModuleJS(name)
		if !containsAll(js, parts...) {
			t.Fatalf("%s module missing local event contract:\n%s", name, js)
		}
	}
}

func TestGeneratedPhase3BaseComponentsRunInBrowser(t *testing.T) {
	if _, err := exec.LookPath("node"); err != nil {
		t.Skip("node not available")
	}
	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatal(err)
	}
	playwrightPackage := os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE")
	if playwrightPackage == "" {
		playwrightPackage = filepath.Join(repoRoot, "lwcruntime", "node_modules", "playwright")
	}
	if _, err := os.Stat(playwrightPackage); err != nil {
		if os.Getenv("GLADE_LWC_PLAYWRIGHT_MODULE") != "" {
			t.Fatalf("configured Playwright module is unavailable: %v", err)
		}
		t.Skip("playwright node module not installed")
	}
	toolchainRoot := repoRoot
	if _, err := os.Stat(filepath.Join(toolchainRoot, "third_party", "lwc", "node_modules", "@lwc", "engine-dom")); err != nil {
		root, err := gladehome.EnsureRoot()
		if err != nil {
			t.Skipf("LWC toolchain node modules not installed: %v", err)
		}
		toolchainRoot = root
	}
	shims := map[string]string{
		"checkboxGroup": LightningBaseComponentModuleJS("checkboxGroup"),
		"dualListbox":   LightningBaseComponentModuleJS("dualListbox"),
		"datatable":     LightningBaseComponentModuleJS("datatable"),
		"treeGrid":      LightningBaseComponentModuleJS("treeGrid"),
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gen.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!DOCTYPE html><html><head>
<script>window.process = { env: { NODE_ENV: "production" } };</script>
<script type="importmap">{"imports":{"lwc":"/lightning/vendor/lwc.js","@lwc/synthetic-shadow":"/lightning/vendor/synthetic-shadow.js","@glade/shell/diagnostics":"/lightning/runtime/shell/diagnostics.js","lightning/checkboxGroup":"/lightning/shims/lightning/checkboxGroup.js","lightning/dualListbox":"/lightning/shims/lightning/dualListbox.js","lightning/datatable":"/lightning/shims/lightning/datatable.js","lightning/treeGrid":"/lightning/shims/lightning/treeGrid.js","lightning/uiRecordApi":"/lightning/shims/lightning/uiRecordApi.js","lightning/uiObjectInfoApi":"/lightning/shims/lightning/uiObjectInfoApi.js","@salesforce/i18n/locale":"/lightning/shims/i18n/locale.js","@salesforce/i18n/currency":"/lightning/shims/i18n/currency.js"}}</script>
</head><body><div id="host"></div><script type="module" src="/entry.js"></script></body></html>`)
		case "/entry.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprintf(w, `
import "@lwc/synthetic-shadow";
import { createElement } from "lwc";
import CheckboxGroup from "lightning/checkboxGroup";
import DualListbox from "lightning/dualListbox";
import Datatable from "lightning/datatable";
import TreeGrid from "lightning/treeGrid";
const host = document.getElementById("host");
function append(tag, Ctor, props = {}) {
  const el = createElement(tag, { is: Ctor });
  Object.assign(el, props);
  host.appendChild(el);
  return el;
}
const checks = append("lightning-checkbox-group", CheckboxGroup, {
  label: "Checks",
  value: ["alpha"],
  options: [{ label: "Alpha", value: "alpha" }, { label: "Beta", value: "beta" }]
});
checks.addEventListener("change", (event) => { window.__checkboxGroup = event.detail; });
const dual = append("lightning-dual-listbox", DualListbox, {
  label: "Providers",
  sourceLabel: "Available",
  selectedLabel: "Selected",
  value: ["alpha"],
  options: [{ label: "Alpha", value: "alpha" }, { label: "Beta", value: "beta" }]
});
dual.addEventListener("change", (event) => { window.__dualListbox = event.detail; });
// Local regression fixture for the pre-existing public-assignment and save
// contracts restored during L20 review; these are not new Salesforce rows.
window.__l20Datatable = append("lightning-datatable", Datatable, {
  keyField: "id",
  columns: [{ label: "Name", fieldName: "Name", type: "text", editable: true }],
  data: [{ id: "r1", Name: "Alpha" }, { id: "r2", Name: "Bravo" }],
  selectedRows: ["r2"],
  draftValues: []
});
window.__l20Saves = [];
window.__l20Datatable.addEventListener("save", (event) => { window.__l20Saves.push(event.detail.draftValues); });
// Local tree-grid review regressions, not additional native oracle rows.
window.__l20TreeGrid = append("lightning-tree-grid", TreeGrid, {
  keyField: "id",
  columns: [{ label: "Name", fieldName: "Name", type: "text" }],
  data: [{ id: "p", Name: "Parent", _children: [{ id: "c", Name: "Child" }] }, { id: "l", Name: "Leaf" }],
  expandedRows: ["p"],
  selectedRows: ["l"]
});
window.__l20GridSelections = [];
window.__l20TreeGrid.addEventListener("rowselection", (event) => { window.__l20GridSelections.push(event.detail); });
`)
		case "/lightning/vendor/lwc.js":
			serveTestFile(t, w, filepath.Join(toolchainRoot, "third_party", "lwc", "node_modules", "@lwc", "engine-dom", "dist", "index.js"))
		case "/lightning/vendor/synthetic-shadow.js":
			serveTestFile(t, w, filepath.Join(toolchainRoot, "third_party", "lwc", "node_modules", "@lwc", "synthetic-shadow", "dist", "index.js"))
		case "/lightning/runtime/shell/diagnostics.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprint(w, `export const diagnostics = []; export function reportDiagnostic(diagnostic) { diagnostics.push(diagnostic); }`)
		case "/lightning/shims/lightning/checkboxGroup.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprint(w, shims["checkboxGroup"])
		case "/lightning/shims/lightning/dualListbox.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprint(w, shims["dualListbox"])
		case "/lightning/shims/lightning/datatable.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprint(w, shims["datatable"])
		case "/lightning/shims/lightning/treeGrid.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprint(w, shims["treeGrid"])
		case "/lightning/runtime/lightning/source/datatable/datatable.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprint(w, readRuntimeLightningFile(t, filepath.Join("source", "datatable", "datatable.js")))
		case "/lightning/runtime/lightning/lds-form.mjs":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprint(w, readRuntimeLightningFile(t, "lds-form.mjs"))
		case "/lightning/shims/lightning/uiRecordApi.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprint(w, UIRecordAPIModuleJS())
		case "/lightning/shims/lightning/uiObjectInfoApi.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprint(w, UIObjectInfoAPIModuleJS())
		case "/lightning/shims/i18n/locale.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprint(w, I18nModuleJS("locale"))
		case "/lightning/shims/i18n/currency.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprint(w, I18nModuleJS("currency"))
		case "/lightning/shims/core/wire-adapter.js":
			serveTestFile(t, w, filepath.Join(repoRoot, "lwcruntime", "src", "shims", "wire-adapter.mjs"))
		case "/lightning/shims/core/lds-cache.mjs":
			serveTestFile(t, w, filepath.Join(repoRoot, "lwcruntime", "src", "shims", "lds-cache.mjs"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	script := fmt.Sprintf(`
import assert from "node:assert/strict";
import { createRequire } from "node:module";
const require = createRequire(import.meta.url);
const { chromium } = require(%q);
const launchOptions = { headless: true };
const browserExecutable = %q;
if (browserExecutable !== "") launchOptions.executablePath = browserExecutable;
const browser = await chromium.launch(launchOptions);
try {
  const page = await browser.newPage();
  const pageErrors = [];
  const consoleErrors = [];
  page.on("pageerror", (err) => pageErrors.push(err.message));
  page.on("console", (msg) => { if (msg.type() === "error") consoleErrors.push(msg.text()); });
  await page.goto(%q, { waitUntil: "networkidle" });
  assert.deepEqual(pageErrors, [], "generated component module errors");
  assert.deepEqual(consoleErrors, [], "generated component resource errors");
  await page.locator('lightning-checkbox-group input[value="beta"]').check();
  assert.deepEqual(await page.evaluate(() => window.__checkboxGroup), { value: ["alpha", "beta"] });
  // L20 dual_move_normal: native options are selectable ARIA listbox items.
  assert.deepEqual(await page.locator('lightning-dual-listbox [data-list="source"] [role="option"]').allTextContents(), ["Beta"]);
  assert.deepEqual(await page.locator('lightning-dual-listbox [data-list="selected"] [role="option"]').allTextContents(), ["Alpha"]);
  await page.locator('lightning-dual-listbox [data-list="source"] [role="option"]').click();
  await page.getByRole("button", { name: "Move selection to Selected" }).click();
  assert.deepEqual(await page.evaluate(() => window.__dualListbox), { value: ["alpha", "beta"] });
  const grid = page.locator("lightning-tree-grid");
  const expandedIDs = () => page.evaluate(() => window.__l20TreeGrid.getCurrentExpandedRows());
  const gridSelectedIDs = () => page.evaluate(() => window.__l20TreeGrid.getSelectedRows().map((row) => row.id));
  const collapseParent = grid.getByRole("button", { name: "Collapse Parent", exact: true });
  const expandParent = grid.getByRole("button", { name: "Expand Parent", exact: true });
  assert.deepEqual(await expandedIDs(), ["p"]);
  await collapseParent.click();
  assert.deepEqual(await expandedIDs(), []);
  await grid.evaluate((element) => { element.expandedRows = element.expandedRows; });
  assert.deepEqual(await expandedIDs(), ["p"]);
  await collapseParent.waitFor({ state: "visible" });
  await grid.evaluate((element) => { element.expandedRows = []; });
  await expandParent.waitFor({ state: "visible" });
  assert.equal(await grid.locator("tbody tr").count(), 2);
  await grid.evaluate((element) => { element.expandAll(); });
  assert.deepEqual(await expandedIDs(), ["p"]);
  await collapseParent.waitFor({ state: "visible" });
  await grid.evaluate((element) => { element.expandedRows = element.expandedRows; });
  assert.deepEqual(await expandedIDs(), []);
  await expandParent.waitFor({ state: "visible" });
  await grid.evaluate((element) => { element.expandedRows = ["p"]; });
  await collapseParent.waitFor({ state: "visible" });
  await grid.evaluate((element) => { element.collapseAll(); });
  assert.deepEqual(await expandedIDs(), []);
  await expandParent.waitFor({ state: "visible" });
  await grid.evaluate((element) => { element.expandedRows = element.expandedRows; });
  assert.deepEqual(await expandedIDs(), ["p"]);
  await collapseParent.waitFor({ state: "visible" });
  await grid.evaluate((element) => { element.expandedRows = []; });
  await expandParent.waitFor({ state: "visible" });

  assert.deepEqual(await gridSelectedIDs(), ["l"]);
  await grid.locator('tbody input[type="checkbox"]').first().check();
  assert.deepEqual(await gridSelectedIDs(), ["p", "l"]);
  await grid.evaluate((element) => { element.selectedRows = element.selectedRows; });
  assert.deepEqual(await gridSelectedIDs(), ["l"]);
  await grid.locator('tbody tr:first-child input[type="checkbox"]:checked').waitFor({ state: "detached" });
  await grid.evaluate((element) => { element.selectedRows = []; });
  assert.deepEqual(await gridSelectedIDs(), []);
  await grid.locator('tbody input[type="checkbox"]:checked').waitFor({ state: "detached" });
  await grid.locator('tbody input[type="checkbox"]').first().check();
  assert.deepEqual(await gridSelectedIDs(), ["p"]);
  assert.deepEqual(await page.evaluate(() => window.__l20GridSelections.at(-1).config), {
    action: "rowSelect", value: "p", selectedRowKeys: ["p"]
  });
  await grid.evaluate((element) => { element.selectedRows = ["l"]; });
  assert.deepEqual(await gridSelectedIDs(), ["l"]);
  await grid.locator('tbody tr:first-child input[type="checkbox"]:checked').waitFor({ state: "detached" });
  // The captured header remains visible; uncaptured bulk selection is removed.
  const gridSelectionCount = await page.evaluate(() => window.__l20GridSelections.length);
  await grid.locator('thead input[type="checkbox"]').click();
  await grid.locator('thead input[type="checkbox"]').click();
  assert.deepEqual(await gridSelectedIDs(), ["l"]);
  assert.equal(await page.evaluate(() => window.__l20GridSelections.length), gridSelectionCount);

  const table = page.locator("lightning-datatable");
  const selectedIDs = () => page.evaluate(() => window.__l20Datatable.getSelectedRows().map((row) => row.id));
  await table.locator('tbody input[type="checkbox"]').first().check();
  assert.deepEqual(await selectedIDs(), ["r1", "r2"]);
  // An assignment wins even when the public array reference is unchanged.
  await table.evaluate((element) => { element.selectedRows = element.selectedRows; });
  assert.deepEqual(await selectedIDs(), ["r2"]);
  await table.evaluate((element) => { element.selectedRows = []; });
  assert.deepEqual(await selectedIDs(), []);
  await table.locator('tbody input[type="checkbox"]:checked').first().waitFor({ state: "detached" });
  await table.evaluate((element) => { element.selectedRows = ["r1"]; });
  await table.locator('tbody input[type="checkbox"]').nth(1).check();
  assert.deepEqual(await selectedIDs(), ["r1", "r2"]);
  await table.evaluate((element) => { element.selectedRows = []; });
  assert.deepEqual(await selectedIDs(), []);

  await table.getByRole("button", { name: "Edit Name", exact: true }).first().click();
  await table.locator('input[data-field-name="Name"]').fill("Edited");
  await table.locator('input[data-field-name="Name"]').press("Enter");
  const save = table.getByRole("button", { name: "Save", exact: true });
  await save.click();
  await save.click();
  assert.deepEqual(await page.evaluate(() => window.__l20Saves), [
    [{ id: "r1", Name: "Edited" }], [{ id: "r1", Name: "Edited" }]
  ]);
  await table.evaluate((element) => { element.draftValues = [{ id: "r1", Name: "Assigned" }]; });
  assert.match(await table.locator("tbody tr").first().innerText(), /Assigned/);
  await table.getByRole("button", { name: "Edit Name", exact: true }).first().click();
  await table.locator('input[data-field-name="Name"]').fill("Edited Again");
  await table.locator('input[data-field-name="Name"]').press("Enter");
  await table.evaluate((element) => { element.draftValues = element.draftValues; });
  assert.match(await table.locator("tbody tr").first().innerText(), /Assigned/);
  await table.evaluate((element) => { element.draftValues = []; });
  await save.waitFor({ state: "detached" });
  assert.match(await table.locator("tbody tr").first().innerText(), /Alpha/);

  // Clearing during the application save handler also invalidates edits.
  await table.getByRole("button", { name: "Edit Name", exact: true }).first().click();
  await table.locator('input[data-field-name="Name"]').fill("App Clears");
  await table.locator('input[data-field-name="Name"]').press("Enter");
  await table.evaluate((element) => { element.addEventListener("save", () => { element.draftValues = []; }, { once: true }); });
  await save.click();
  await save.waitFor({ state: "detached" });
  assert.match(await table.locator("tbody tr").first().innerText(), /Alpha/);
  assert.deepEqual(pageErrors, []);
  assert.deepEqual(consoleErrors, []);
} finally {
  await browser.close();
}
`, playwrightPackage, os.Getenv("GLADE_LWC_BROWSER_EXECUTABLE"), server.URL+"/gen.html")
	if err := os.WriteFile(filepath.Join(dir, "test.mjs"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "test.mjs")
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), "TMPDIR="+dir, "TMP="+dir, "TEMP="+dir)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("generated base component browser test failed: %v\n%s", err, output)
	}
}

func serveTestFile(t *testing.T, w http.ResponseWriter, path string) {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
	_, _ = w.Write(data)
}
