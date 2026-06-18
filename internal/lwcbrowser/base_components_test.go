package lwcbrowser

import (
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
		if !containsAll(js, "registerComponent", "export default", "createBaseComponent") {
			t.Fatalf("%s module js = %q", name, js)
		}
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
		if !containsAll(js, "registerComponent", "export default", "createBaseComponent") {
			t.Fatalf("%s module js = %q", name, js)
		}
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
		if !containsAll(js, "registerComponent", "export default", "createBaseComponent") {
			t.Fatalf("%s module js = %q", name, js)
		}
		if strings.Contains(js, "GLADELWC060") {
			t.Fatalf("%s should be supported for phase 3, got diagnostic js = %q", name, js)
		}
	}
}

func TestUnsupportedLightningBaseComponentEmitsDiagnostic(t *testing.T) {
	if !IsLightningBaseComponentModule("formattedLocation") {
		t.Fatalf("formattedLocation should be recognized so the browser receives a diagnostic module")
	}
	js := LightningBaseComponentModuleJS("formattedLocation")
	if !containsAll(js, "GLADELWC060", "base component unsupported", "lightning-formatted-location") {
		t.Fatalf("unsupported component js = %q", js)
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
		"progressRing":   "lightning-progress-ring",
		"treeGrid":       "lightning-tree-grid",
	}
	for name, want := range cases {
		if got := LightningBaseComponentModuleJS(name); !strings.Contains(got, want) {
			t.Fatalf("%s module missing %s:\n%s", name, want, got)
		}
	}
}

func TestBaseComponentPublicPropsIncludeFieldName(t *testing.T) {
	js := LightningBaseComponentModuleJS("outputField")
	if !containsAll(js, `"fieldName"`, "$cmp.fieldName") {
		t.Fatalf("outputField module missing fieldName support:\n%s", js)
	}
}

func TestBaseComponentPublicMethodsExposeFormAndValidityContracts(t *testing.T) {
	js := LightningBaseComponentModuleJS("inputField")
	if !containsAll(js,
		"publicMethods: basePublicMethods()",
		`"setErrors"`,
		`"getErrors"`,
		`"wireRecordUi"`,
		`"getWiredData"`,
		`"wirePicklistValues"`,
		`"getWiredPicklistValues"`,
		`"setValue"`,
		`"clean"`,
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
	js := LightningBaseComponentModuleJS("datatable")
	if !containsAll(js, "handleRowAction", `"rowaction"`, "typeAttributes", "rowActions") {
		t.Fatalf("datatable module missing row action support:\n%s", js)
	}
}

func TestRecordFormModuleUsesLocalLDSEndpoints(t *testing.T) {
	js := LightningBaseComponentModuleJS("recordEditForm")
	if !containsAll(js,
		"loadRecordFormRecord",
		`"/lightning/wire/getRecord"`,
		`"/lightning/wire/updateRecord"`,
		`"success"`,
		`"data-field-name"`,
		"recordFieldDisplayValue",
	) {
		t.Fatalf("record form module missing LDS-backed rendering and submit support:\n%s", js)
	}
}

func TestTabModuleDispatchesActiveEvent(t *testing.T) {
	js := LightningBaseComponentModuleJS("tab")
	if !containsAll(js, "handleActive", `"active"`, "this.value", "this.label") {
		t.Fatalf("tab module missing active event support:\n%s", js)
	}
}

func TestBaseComponentModuleReportsUnsupportedAttributes(t *testing.T) {
	js := LightningBaseComponentModuleJS("datatable")
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

func TestDatatableModuleKeepsActionColumnIndex(t *testing.T) {
	js := LightningBaseComponentModuleJS("datatable")
	if !containsAll(js, "columnIndex", "dataset.columnIndex", `"data-column-index"`) {
		t.Fatalf("datatable module should preserve action column index:\n%s", js)
	}
}

func TestModalModuleProvidesOpenStatic(t *testing.T) {
	js := LightningBaseComponentModuleJS("modal")
	if !containsAll(js, "static async open", "lightning__modalopen", "options.result") {
		t.Fatalf("modal module missing static open approximation:\n%s", js)
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
	if _, err := os.Stat(filepath.Join(repoRoot, "lwcruntime", "node_modules", "playwright")); err != nil {
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
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/gen.html":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			fmt.Fprintf(w, `<!DOCTYPE html><html><head>
<script>window.process = { env: { NODE_ENV: "production" } };</script>
<script type="importmap">{"imports":{"lwc":"/lightning/vendor/lwc.js","@lwc/synthetic-shadow":"/lightning/vendor/synthetic-shadow.js","@glade/shell/diagnostics":"/lightning/runtime/shell/diagnostics.js","lightning/checkboxGroup":"/lightning/shims/lightning/checkboxGroup.js","lightning/dualListbox":"/lightning/shims/lightning/dualListbox.js"}}</script>
</head><body><div id="host"></div><script type="module" src="/entry.js"></script></body></html>`)
		case "/entry.js":
			w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
			fmt.Fprintf(w, `
import "@lwc/synthetic-shadow";
import { createElement } from "lwc";
import CheckboxGroup from "lightning/checkboxGroup";
import DualListbox from "lightning/dualListbox";
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
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	dir := t.TempDir()
	script := fmt.Sprintf(`
import assert from "node:assert/strict";
import { createRequire } from "node:module";
const require = createRequire(%q);
const { chromium } = require("playwright");
const browser = await chromium.launch({ headless: true });
try {
  const page = await browser.newPage();
  const pageErrors = [];
  const consoleErrors = [];
  page.on("pageerror", (err) => pageErrors.push(err.message));
  page.on("console", (msg) => { if (msg.type() === "error") consoleErrors.push(msg.text()); });
  await page.goto(%q, { waitUntil: "networkidle" });
  await page.locator('lightning-checkbox-group input[value="beta"]').check();
  assert.deepEqual(await page.evaluate(() => window.__checkboxGroup), { value: ["alpha", "beta"] });
  assert.deepEqual(await page.locator('lightning-dual-listbox select[data-list="source"] option').allTextContents(), ["Beta"]);
  assert.deepEqual(await page.locator('lightning-dual-listbox select[data-list="selected"] option').allTextContents(), ["Alpha"]);
  await page.locator('lightning-dual-listbox select[data-list="source"]').selectOption("beta");
  await page.getByRole("button", { name: "Move selection to Selected" }).click();
  assert.deepEqual(await page.evaluate(() => window.__dualListbox), { value: ["alpha", "beta"] });
  assert.deepEqual(pageErrors, []);
  assert.deepEqual(consoleErrors, []);
} finally {
  await browser.close();
}
`, filepath.Join(repoRoot, "lwcruntime", "package.json"), server.URL+"/gen.html")
	if err := os.WriteFile(filepath.Join(dir, "test.mjs"), []byte(script), 0o644); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", "test.mjs")
	cmd.Dir = dir
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
