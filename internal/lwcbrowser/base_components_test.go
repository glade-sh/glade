package lwcbrowser

import (
	"strings"
	"testing"
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

func TestUnsupportedLightningBaseComponentEmitsDiagnostic(t *testing.T) {
	if !IsLightningBaseComponentModule("accordion") {
		t.Fatalf("accordion should be recognized so the browser receives a diagnostic module")
	}
	js := LightningBaseComponentModuleJS("accordion")
	if !containsAll(js, "GLADELWC060", "base component unsupported", "lightning-accordion") {
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
