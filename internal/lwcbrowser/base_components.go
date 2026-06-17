package lwcbrowser

import (
	"encoding/json"
	"fmt"
	"strings"
)

type baseComponentDefinition struct {
	Name       string
	Tag        string
	ClassName  string
	Tier       int
	Supported  bool
	TemplateJS string
}

var lightningBaseComponentDefinitions = map[string]baseComponentDefinition{}

func init() {
	for _, def := range []baseComponentDefinition{
		baseComponent("button", 1, true, buttonTemplateJS()),
		baseComponent("buttonIcon", 1, true, iconButtonTemplateJS()),
		baseComponent("card", 1, true, cardTemplateJS()),
		baseComponent("input", 1, true, inputTemplateJS()),
		baseComponent("textarea", 1, true, textareaTemplateJS()),
		baseComponent("combobox", 1, true, comboboxTemplateJS()),
		baseComponent("layout", 1, true, layoutTemplateJS()),
		baseComponent("layoutItem", 1, true, layoutItemTemplateJS()),
		baseComponent("tabset", 1, true, tabsetTemplateJS()),
		baseComponent("tab", 1, true, tabTemplateJS()),
		baseComponent("spinner", 1, true, spinnerTemplateJS()),
		baseComponent("icon", 1, true, iconTemplateJS()),
		baseComponent("datatable", 2, true, datatableTemplateJS()),
		baseComponent("recordForm", 2, true, recordFormTemplateJS()),
		baseComponent("recordViewForm", 2, true, recordViewFormTemplateJS()),
		baseComponent("recordEditForm", 2, true, recordEditFormTemplateJS()),
		baseComponent("outputField", 2, true, outputFieldTemplateJS()),
		baseComponent("inputField", 2, true, inputFieldTemplateJS()),
		baseComponent("messages", 2, true, messagesTemplateJS()),
		baseComponent("modal", 2, true, modalTemplateJS()),
	} {
		lightningBaseComponentDefinitions[normalizeLightningBaseComponentName(def.Name)] = def
	}
	for _, name := range unsupportedLightningBaseComponentNames() {
		key := normalizeLightningBaseComponentName(name)
		if _, ok := lightningBaseComponentDefinitions[key]; ok {
			continue
		}
		lightningBaseComponentDefinitions[key] = baseComponent(name, 0, false, "[]")
	}
}

func baseComponent(name string, tier int, supported bool, templateJS string) baseComponentDefinition {
	return baseComponentDefinition{
		Name:       name,
		Tag:        "lightning-" + kebabLightningBaseComponentName(name),
		ClassName:  lightningBaseComponentClassName(name),
		Tier:       tier,
		Supported:  supported,
		TemplateJS: templateJS,
	}
}

func SupportedLightningBaseComponentSpecifiers() map[string]string {
	out := make(map[string]string)
	for _, def := range lightningBaseComponentDefinitions {
		if !def.Supported {
			continue
		}
		out["lightning/"+def.Name] = "/lightning/shims/lightning/" + def.Name + ".js"
	}
	return out
}

func IsLightningBaseComponentModule(name string) bool {
	_, ok := lightningBaseComponentDefinitions[normalizeLightningBaseComponentName(name)]
	return ok
}

func LightningBaseComponentModuleJS(name string) string {
	def, ok := lightningBaseComponentDefinitions[normalizeLightningBaseComponentName(name)]
	if !ok {
		def = baseComponent(name, 0, false, "[]")
	}
	if !def.Supported {
		return unsupportedBaseComponentModuleJS(def)
	}
	classExtraJS := ""
	if normalizeLightningBaseComponentName(def.Name) == "modal" {
		classExtraJS = `  static async open(options = {}) {
    const detail = { ...options };
    window.dispatchEvent(new CustomEvent("lightning__modalopen", { detail }));
    return options.result;
  }
`
	}
	return fmt.Sprintf(`import { LightningElement, registerDecorators, registerTemplate, freezeTemplate, registerComponent } from "lwc";
import { reportDiagnostic } from "@glade/shell/diagnostics";
function createBaseComponent() {}
function tmpl($api, $cmp, $slotset, $ctx) {
  const { h: api_element, t: api_text, d: api_dynamic_text, b: api_bind, s: api_slot } = $api;
  return %[1]s;
}
tmpl.stylesheets = [];
tmpl.slots = [""];
const template = registerTemplate(tmpl);
freezeTemplate(tmpl);
class %[2]s extends LightningElement {
%[4]s
  connectedCallback() {
    this.reportUnsupportedAttributes();
    if (isRecordFormSelector(%[3]q)) {
      this.loadRecordFormRecord();
    }
  }
  reportUnsupportedAttributes() {
    const unsupportedAttrs = unsupportedBaseAttributes(this);
    if (!unsupportedAttrs.length) {
      return;
    }
    const message = "GLADELWC061 base component attributes unsupported locally: " + unsupportedAttrs.join(", ");
    reportDiagnostic({ code: "GLADELWC061", severity: "warning", message, tagName: %[3]q, attributes: unsupportedAttrs });
  }
  handleChange(event) {
    const target = event && event.target || {};
    this.value = target.value;
    this.checked = target.checked;
    this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value: target.value, checked: target.checked } }));
  }
  handleSubmit(event) {
    event.preventDefault();
    const fields = this.collectRecordFormFields();
    const submitEvent = new CustomEvent("submit", { bubbles: true, composed: true, cancelable: true, detail: { fields } });
    this.dispatchEvent(submitEvent);
    if (!isRecordFormSelector(%[3]q) || submitEvent.defaultPrevented) {
      return;
    }
    fetch("/lightning/wire/updateRecord", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ fields: { Id: this.recordId, ...fields } })
    }).then((response) => response.json()).then((result) => {
      if (result && result.error) {
        this.error = result.error;
        this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail: result.error }));
        return;
      }
      if (result && result.data) {
        this.value = result.data;
      }
      this.dispatchEvent(new CustomEvent("success", { bubbles: true, composed: true, detail: { id: result && result.data && result.data.id || this.recordId, fields } }));
    }).catch((err) => {
      const detail = { message: err && err.message || String(err) };
      this.error = detail;
      this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail }));
    });
  }
  collectRecordFormFields() {
    const fields = {};
    const inputs = this.template && this.template.querySelectorAll ? this.template.querySelectorAll("[data-field-name]") : [];
    for (const input of inputs) {
      const name = input.dataset && input.dataset.fieldName;
      if (!name) {
        continue;
      }
      fields[name] = input.type === "checkbox" ? Boolean(input.checked) : input.value;
    }
    return fields;
  }
  loadRecordFormRecord() {
    if (!this.objectApiName || !this.recordId || this.__recordFormLoaded) {
      return;
    }
    this.__recordFormLoaded = true;
    fetch("/lightning/wire/getRecord", {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({ recordId: this.recordId, fields: recordFormFieldRefs(this.objectApiName, this.fields) })
    }).then((response) => response.json()).then((result) => {
      if (result && result.error) {
        this.error = result.error;
        this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail: result.error }));
        return;
      }
      this.value = result && result.data || result;
    }).catch((err) => {
      const detail = { message: err && err.message || String(err) };
      this.error = detail;
      this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail }));
    });
  }
  handleRowAction(event) {
    const dataset = event && event.currentTarget && event.currentTarget.dataset || {};
    const rowIndex = Number(dataset.rowIndex);
    const columnIndex = Number(dataset.columnIndex);
    const actionIndex = Number(dataset.actionIndex);
    const rows = this.data || [];
    const column = (this.columns || [])[columnIndex];
    const actions = column && column.typeAttributes && column.typeAttributes.rowActions || [];
    const row = rows[rowIndex];
    const action = actions[actionIndex];
    if (!row || !action) {
      return;
    }
    this.dispatchEvent(new CustomEvent("rowaction", { bubbles: true, composed: true, detail: { action, row } }));
  }
  handleActive(event) {
    if (event && event.preventDefault) {
      event.preventDefault();
    }
    this.dispatchEvent(new CustomEvent("active", { bubbles: true, composed: true, detail: { value: this.value || this.name || this.label || "", label: this.label || "" } }));
  }
}
registerDecorators(%[2]s, { publicProps: basePublicProps() });
function basePublicProps() {
  const props = {};
  for (const name of ["label","title","value","options","checked","disabled","type","variant","iconName","alternativeText","size","columns","data","keyField","objectApiName","recordId","fields","mode","name","fieldName","error"]) {
    props[name] = { config: 0 };
  }
  return props;
}
function unsupportedBaseAttributes(component) {
  const host = component && component.hostElement || component;
  if (!host || typeof host.getAttributeNames !== "function") {
    return [];
  }
  const unsupportedAttrs = [];
  const known = new Set(["hide-checkbox-column", "max-row-selection", "sorted-by", "sorted-direction", "show-row-number-column", "wrap-text-max-lines"]);
  for (const name of host.getAttributeNames()) {
    if (known.has(name)) {
      unsupportedAttrs.push(name);
    }
  }
  return unsupportedAttrs;
}
function fieldList(fields) {
  if (Array.isArray(fields)) {
    return fields;
  }
  if (typeof fields === "string" && fields.trim()) {
    return fields.split(",").map((field) => field.trim());
  }
  return [];
}
function isRecordFormSelector(selector) {
  return selector === "lightning-record-form" || selector === "lightning-record-view-form" || selector === "lightning-record-edit-form";
}
function fieldApiName(field) {
  const value = typeof field === "string" ? field : field && (field.fieldApiName || field.apiName || field.fieldName || field.name) || "";
  const parts = String(value || "").split(".");
  return parts[parts.length - 1] || "";
}
function recordFormFieldRefs(objectApiName, fields) {
  return fieldList(fields).map((field) => {
    const raw = typeof field === "string" ? field : field && (field.fieldApiName || field.apiName || field.fieldName || field.name) || "";
    if (!raw) {
      return "";
    }
    if (String(raw).includes(".")) {
      return String(raw);
    }
    return objectApiName + "." + raw;
  }).filter(Boolean);
}
function recordFieldDisplayValue(record, field) {
  const name = fieldApiName(field);
  const value = record && record.fields && record.fields[name];
  if (value && typeof value === "object") {
    return value.displayValue ?? value.value ?? "";
  }
  return value ?? "";
}
export default registerComponent(%[2]s, { tmpl: template, sel: %[3]q });
`, def.TemplateJS, def.ClassName, def.Tag, classExtraJS)
}

func normalizeLightningBaseComponentName(name string) string {
	name = strings.TrimSpace(name)
	name = strings.TrimSuffix(name, ".js")
	name = strings.ReplaceAll(name, "-", "")
	name = strings.ReplaceAll(name, "_", "")
	return strings.ToLower(name)
}

func kebabLightningBaseComponentName(name string) string {
	name = strings.TrimSpace(strings.TrimSuffix(name, ".js"))
	if name == "" {
		return "component"
	}
	var out strings.Builder
	lastDash := false
	for i, r := range name {
		if r == '_' || r == '-' || r == ' ' {
			if !lastDash && out.Len() > 0 {
				out.WriteByte('-')
				lastDash = true
			}
			continue
		}
		if r >= 'A' && r <= 'Z' {
			if i > 0 && !lastDash {
				out.WriteByte('-')
			}
			r += 'a' - 'A'
		}
		out.WriteRune(r)
		lastDash = false
	}
	value := strings.Trim(out.String(), "-")
	if value == "" {
		return "component"
	}
	return value
}

func lightningBaseComponentClassName(name string) string {
	kebab := kebabLightningBaseComponentName(name)
	var out strings.Builder
	out.WriteString("Lightning")
	upperNext := true
	for _, r := range kebab {
		if r == '-' {
			upperNext = true
			continue
		}
		if upperNext && r >= 'a' && r <= 'z' {
			r -= 'a' - 'A'
		}
		out.WriteRune(r)
		upperNext = false
	}
	if out.String() == "Lightning" {
		return "LightningComponent"
	}
	return out.String()
}

func unsupportedBaseComponentModuleJS(def baseComponentDefinition) string {
	message := "GLADELWC060 base component unsupported: " + def.Tag
	raw, err := json.Marshal(message)
	if err != nil {
		raw = []byte(`"GLADELWC060 base component unsupported"`)
	}
	return fmt.Sprintf(`import { reportDiagnostic } from "@glade/shell/diagnostics";
const message = %s;
reportDiagnostic({ code: "GLADELWC060", severity: "warning", message, module: %q, tagName: %q });
throw new Error(message);
export default undefined;
`, raw, def.Name, def.Tag)
}

func unsupportedLightningBaseComponentNames() []string {
	return []string{
		"accordion", "accordionSection", "avatar", "badge", "buttonGroup", "buttonIconStateful",
		"buttonMenu", "checkboxGroup", "dualListbox", "formattedAddress", "formattedDateTime",
		"formattedEmail", "formattedLocation", "formattedName", "formattedNumber", "formattedPhone",
		"formattedRichText", "formattedText", "formattedTime", "formattedUrl", "helptext",
		"inputAddress", "inputLocation", "menuItem", "menuSubheader", "pill", "pillContainer",
		"progressBar", "progressIndicator", "progressRing", "radioGroup", "select", "slider",
		"tile", "tree", "treeGrid", "verticalNavigation",
	}
}

func buttonTemplateJS() string {
	return `[api_element("button", { classMap: { "slds-button": true, "slds-button_neutral": true }, attrs: { type: $cmp.type || "button" }, props: { disabled: Boolean($cmp.disabled) }, key: 0 }, [api_text($cmp.label || "")])]`
}

func iconButtonTemplateJS() string {
	return `[api_element("button", { classMap: { "slds-button": true, "slds-button_icon": true }, attrs: { type: "button", title: $cmp.alternativeText || $cmp.iconName || "" }, props: { disabled: Boolean($cmp.disabled) }, key: 0 }, [api_text(($cmp.iconName || "utility:button").split(":").pop())])]`
}

func cardTemplateJS() string {
	return `[api_element("article", { classMap: { "slds-card": true }, key: 0 }, [api_element("header", { classMap: { "slds-card__header": true }, key: 1 }, [api_element("h2", { classMap: { "slds-card__header-title": true }, key: 2 }, [api_text($cmp.title || "")])]), api_element("div", { classMap: { "slds-card__body": true }, key: 3 }, [api_slot("", { key: 4 }, [], $slotset)])])]`
}

func inputTemplateJS() string {
	return `[api_element("label", { classMap: { "slds-form-element": true }, key: 0 }, [api_element("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [api_text($cmp.label || "")]), api_element("input", { classMap: { "slds-input": true }, attrs: { type: $cmp.type || "text" }, props: { value: $cmp.value || "", disabled: Boolean($cmp.disabled) }, key: 2, on: { change: api_bind($cmp.handleChange), input: api_bind($cmp.handleChange) } })])]`
}

func textareaTemplateJS() string {
	return `[api_element("label", { classMap: { "slds-form-element": true }, key: 0 }, [api_element("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [api_text($cmp.label || "")]), api_element("textarea", { classMap: { "slds-textarea": true }, props: { value: $cmp.value || "" }, key: 2, on: { change: api_bind($cmp.handleChange), input: api_bind($cmp.handleChange) } })])]`
}

func comboboxTemplateJS() string {
	return `[api_element("label", { classMap: { "slds-form-element": true, "slds-combobox": true }, key: 0 }, [api_element("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [api_text($cmp.label || "")]), api_element("select", { classMap: { "slds-select": true }, props: { value: $cmp.value || "" }, key: 2, on: { change: api_bind($cmp.handleChange) } }, ($cmp.options || []).map((option, index) => api_element("option", { attrs: { value: String(option.value ?? option.label ?? "") }, props: { selected: String(option.value ?? option.label ?? "") === String($cmp.value ?? "") }, key: 20 + index }, [api_text(option.label || option.value || "")])))])]`
}

func layoutTemplateJS() string {
	return `[api_element("div", { classMap: { "slds-grid": true, "slds-wrap": true }, key: 0 }, [api_slot("", { key: 1 }, [], $slotset)])]`
}

func layoutItemTemplateJS() string {
	return `[api_element("div", { classMap: { "slds-col": true }, key: 0 }, [api_slot("", { key: 1 }, [], $slotset)])]`
}

func tabsetTemplateJS() string {
	return `[api_element("div", { classMap: { "slds-tabs_default": true }, key: 0 }, [api_element("div", { classMap: { "slds-tabs_default__content": true }, key: 1 }, [api_slot("", { key: 2 }, [], $slotset)])])]`
}

func tabTemplateJS() string {
	return `[api_element("section", { classMap: { "slds-tabs_default__content": true }, key: 0 }, [api_element("h3", { classMap: { "slds-tabs_default__item": true }, attrs: { role: "tab", tabindex: "0" }, key: 1, on: { click: api_bind($cmp.handleActive) } }, [api_text($cmp.label || "")]), api_slot("", { key: 2 }, [], $slotset)])]`
}

func spinnerTemplateJS() string {
	return `[api_element("div", { classMap: { "slds-spinner": true }, attrs: { role: "status" }, key: 0 }, [api_text($cmp.alternativeText || "Loading")])]`
}

func iconTemplateJS() string {
	return `[api_element("span", { classMap: { "slds-icon": true }, attrs: { title: $cmp.alternativeText || $cmp.iconName || "" }, key: 0 }, [api_text(($cmp.iconName || "utility:placeholder").split(":").pop())])]`
}

func datatableTemplateJS() string {
	return `[api_element("table", { classMap: { "slds-table": true, "slds-table_cell-buffer": true }, key: 0 }, [api_element("thead", { key: 1 }, [api_element("tr", { key: 2 }, ($cmp.columns || []).map((column, index) => api_element("th", { key: 20 + index }, [api_text(column.label || column.fieldName || "")])))]), api_element("tbody", { key: 3 }, ($cmp.data || []).map((row, rowIndex) => api_element("tr", { key: 100 + rowIndex }, ($cmp.columns || []).map((column, colIndex) => api_element("td", { key: 1000 + rowIndex * 50 + colIndex }, column.type === "action" ? ((column.typeAttributes && column.typeAttributes.rowActions) || []).map((action, actionIndex) => api_element("button", { classMap: { "slds-button": true, "slds-button_neutral": true }, attrs: { type: "button", "data-row-index": String(rowIndex), "data-column-index": String(colIndex), "data-action-index": String(actionIndex) }, key: 2000 + rowIndex * 50 + actionIndex, on: { click: api_bind($cmp.handleRowAction) } }, [api_text(action.label || action.name || "Action")])) : [api_text(row[column.fieldName] ?? "")])))))])]`
}

func recordFormTemplateJS() string {
	return `(() => {
  const editMode = $cmp.mode === "edit";
  const children = [
    api_element("div", { classMap: { "slds-text-title_caps": true }, key: 1 }, [api_text(($cmp.objectApiName || "Record") + " " + ($cmp.recordId || ""))]),
    api_element("div", { key: 2 }, fieldList($cmp.fields).map((field, index) => {
      const name = fieldApiName(field);
      const value = recordFieldDisplayValue($cmp.value, field);
      const fieldChildren = [
        api_element("span", { classMap: { "slds-form-element__label": true }, key: 200 + index }, [api_text(name || String(field))])
      ];
      if (editMode) {
        fieldChildren.push(api_element("input", { classMap: { "slds-input": true }, attrs: { type: "text", "data-field-name": name }, props: { value: value || "" }, key: 400 + index }));
      } else {
        fieldChildren.push(api_element("div", { classMap: { "slds-form-element__static": true }, key: 400 + index }, [api_text(value)]));
      }
      return api_element("div", { classMap: { "slds-form-element": true }, key: 20 + index }, fieldChildren);
    }))
  ];
  if (editMode) {
    children.push(api_element("button", { classMap: { "slds-button": true, "slds-button_brand": true }, attrs: { type: "submit" }, key: 3 }, [api_text("Save")]));
  }
  children.push(api_slot("", { key: 4 }, [], $slotset));
  return [api_element("form", { classMap: { "slds-form": true }, attrs: { "data-object-api-name": $cmp.objectApiName || "" }, key: 0, on: { submit: api_bind($cmp.handleSubmit) } }, children)];
})()`
}

func recordViewFormTemplateJS() string {
	return recordFormTemplateJS()
}

func recordEditFormTemplateJS() string {
	return recordFormTemplateJS()
}

func outputFieldTemplateJS() string {
	return `[api_element("div", { classMap: { "slds-form-element": true }, key: 0 }, [api_element("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [api_text($cmp.fieldName || $cmp.name || "")]), api_element("div", { classMap: { "slds-form-element__static": true }, key: 2 }, [api_text($cmp.value || "")])])]`
}

func inputFieldTemplateJS() string {
	return inputTemplateJS()
}

func messagesTemplateJS() string {
	return `[api_element("div", { classMap: { "slds-notify": true, "slds-notify_alert": true }, attrs: { role: "status" }, key: 0 }, [api_slot("", { key: 1 }, [], $slotset)])]`
}

func modalTemplateJS() string {
	return `[api_element("section", { classMap: { "slds-modal": true, "slds-fade-in-open": true }, attrs: { role: "dialog", "aria-modal": "true" }, key: 0 }, [api_element("div", { classMap: { "slds-modal__container": true }, key: 1 }, [api_element("header", { classMap: { "slds-modal__header": true }, key: 2 }, [api_text($cmp.label || $cmp.title || "")]), api_element("div", { classMap: { "slds-modal__content": true }, key: 3 }, [api_slot("", { key: 4 }, [], $slotset)])])])]`
}
