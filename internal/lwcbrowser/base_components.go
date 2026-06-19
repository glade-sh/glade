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
		baseComponent("accordion", 3, true, slotContainerTemplateJS("section", "slds-accordion")),
		baseComponent("accordionSection", 3, true, titledSlotTemplateJS("section", "slds-accordion__section")),
		baseComponent("alert", 3, true, dialogNoticeTemplateJS("alert")),
		baseComponent("avatar", 3, true, avatarTemplateJS()),
		baseComponent("badge", 3, true, textContainerTemplateJS("span", "slds-badge")),
		baseComponent("barcodeScanner", 3, true, barcodeScannerTemplateJS()),
		baseComponent("baseFormattedText", 3, true, textContainerTemplateJS("span", "slds-truncate")),
		baseComponent("breadcrumb", 3, true, breadcrumbTemplateJS()),
		baseComponent("breadcrumbs", 3, true, breadcrumbsTemplateJS()),
		baseComponent("buttonGroup", 3, true, slotContainerTemplateJS("div", "slds-button-group")),
		baseComponent("buttonIconStateful", 3, true, iconButtonTemplateJS()),
		baseComponent("buttonMenu", 3, true, buttonMenuTemplateJS()),
		baseComponent("buttonStateful", 3, true, buttonStatefulTemplateJS()),
		baseComponent("carousel", 3, true, slotContainerTemplateJS("section", "slds-carousel")),
		baseComponent("carouselImage", 3, true, carouselImageTemplateJS()),
		baseComponent("checkboxGroup", 3, true, optionGroupTemplateJS("checkbox")),
		baseComponent("dialog", 3, true, dialogNoticeTemplateJS("dialog")),
		baseComponent("dualListbox", 3, true, dualListboxTemplateJS()),
		baseComponent("dynamicIcon", 3, true, dynamicIconTemplateJS()),
		baseComponent("fileUpload", 3, true, fileUploadTemplateJS()),
		baseComponent("flow", 3, true, flowTemplateJS()),
		baseComponent("focusTrap", 3, true, slotContainerTemplateJS("div", "slds-is-relative")),
		baseComponent("formattedAddress", 3, true, formattedAddressTemplateJS()),
		baseComponent("formattedDateTime", 3, true, textContainerTemplateJS("time", "slds-truncate")),
		baseComponent("formattedEmail", 3, true, formattedEmailTemplateJS()),
		baseComponent("formattedLocation", 3, true, formattedLocationTemplateJS()),
		baseComponent("formattedLookup", 3, true, formattedLookupTemplateJS()),
		baseComponent("formattedName", 3, true, formattedNameTemplateJS()),
		baseComponent("formattedNumber", 3, true, formattedNumberTemplateJS()),
		baseComponent("formattedPhone", 3, true, formattedLinkTemplateJS("tel")),
		baseComponent("formattedRichText", 3, true, textContainerTemplateJS("span", "slds-rich-text-editor__output")),
		baseComponent("formattedText", 3, true, textContainerTemplateJS("span", "slds-truncate")),
		baseComponent("formattedTime", 3, true, textContainerTemplateJS("time", "slds-truncate")),
		baseComponent("formattedUrl", 3, true, formattedLinkTemplateJS("url")),
		baseComponent("groupedCombobox", 3, true, comboboxTemplateJS()),
		baseComponent("helptext", 3, true, helptextTemplateJS()),
		baseComponent("inputAddress", 3, true, inputAddressTemplateJS()),
		baseComponent("inputLocation", 3, true, inputLocationTemplateJS()),
		baseComponent("inputName", 3, true, inputNameTemplateJS()),
		baseComponent("inputRichText", 3, true, inputRichTextTemplateJS()),
		baseComponent("lookupAddress", 3, true, inputAddressTemplateJS()),
		baseComponent("map", 3, true, mapTemplateJS()),
		baseComponent("menuDivider", 3, true, menuDividerTemplateJS()),
		baseComponent("menuItem", 3, true, menuItemTemplateJS()),
		baseComponent("menuSubheader", 3, true, textContainerTemplateJS("h3", "slds-dropdown__header")),
		baseComponent("modalBody", 3, true, slotContainerTemplateJS("div", "slds-modal__content")),
		baseComponent("modalFooter", 3, true, slotContainerTemplateJS("footer", "slds-modal__footer")),
		baseComponent("modalHeader", 3, true, modalHeaderTemplateJS()),
		baseComponent("multiColumnSortingModal", 3, true, titledSlotTemplateJS("section", "slds-modal")),
		baseComponent("overlay", 3, true, slotContainerTemplateJS("section", "slds-popover")),
		baseComponent("picklist", 3, true, selectTemplateJS()),
		baseComponent("pill", 3, true, pillTemplateJS()),
		baseComponent("pillContainer", 3, true, pillContainerTemplateJS()),
		baseComponent("popup", 3, true, slotContainerTemplateJS("section", "slds-popover")),
		baseComponent("primitiveFigure", 3, true, primitiveFigureTemplateJS()),
		baseComponent("progressBar", 3, true, progressBarTemplateJS()),
		baseComponent("progressIndicator", 3, true, slotContainerTemplateJS("ol", "slds-progress__list")),
		baseComponent("progressRing", 3, true, progressRingTemplateJS()),
		baseComponent("progressStep", 3, true, textContainerTemplateJS("li", "slds-progress__item")),
		baseComponent("prompt", 3, true, dialogNoticeTemplateJS("prompt")),
		baseComponent("quickActionPanel", 3, true, quickActionPanelTemplateJS()),
		baseComponent("radioGroup", 3, true, optionGroupTemplateJS("radio")),
		baseComponent("recordPicker", 3, true, recordPickerTemplateJS()),
		baseComponent("relativeDateTime", 3, true, relativeDateTimeTemplateJS()),
		baseComponent("select", 3, true, selectTemplateJS()),
		baseComponent("slider", 3, true, sliderTemplateJS()),
		baseComponent("tile", 3, true, tileTemplateJS()),
		baseComponent("stackedTab", 3, true, stackedTabTemplateJS()),
		baseComponent("stackedTabset", 3, true, slotContainerTemplateJS("div", "slds-tabs_mobile")),
		baseComponent("toast", 3, true, toastTemplateJS()),
		baseComponent("toastContainer", 3, true, slotContainerTemplateJS("section", "slds-notify_container")),
		baseComponent("tree", 3, true, treeTemplateJS()),
		baseComponent("treeGrid", 3, true, treeGridTemplateJS()),
		baseComponent("verticalNavigation", 3, true, slotContainerTemplateJS("nav", "slds-nav-vertical")),
		baseComponent("verticalNavigationItem", 3, true, verticalNavigationItemTemplateJS()),
		baseComponent("verticalNavigationItemBadge", 3, true, verticalNavigationItemBadgeTemplateJS()),
		baseComponent("verticalNavigationItemIcon", 3, true, verticalNavigationItemIconTemplateJS()),
		baseComponent("verticalNavigationOverflow", 3, true, verticalNavigationOverflowTemplateJS()),
		baseComponent("verticalNavigationSection", 3, true, titledSlotTemplateJS("section", "slds-nav-vertical__section")),
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
	if js, ok := LightningSourceBackedComponentModuleJS(def.Name); ok {
		return js
	}
	classExtraJS := ""
	switch normalizeLightningBaseComponentName(def.Name) {
	case "alert":
		classExtraJS = `  static open(options = {}) {
    window.dispatchEvent(new CustomEvent("gladealert", { detail: options, bubbles: true, composed: true }));
    return Promise.resolve(options.result);
  }
`
	case "modal":
		classExtraJS = `  static async open(options = {}) {
    const detail = { ...options };
    window.dispatchEvent(new CustomEvent("lightning__modalopen", { detail }));
    return options.result;
  }
`
	case "prompt":
		classExtraJS = `  static open(options = {}) {
    window.dispatchEvent(new CustomEvent("gladeprompt", { detail: options, bubbles: true, composed: true }));
    return Promise.resolve(options.value ?? options.defaultValue ?? "");
  }
`
	case "toast":
		classExtraJS = `  static show(config = {}, source) {
    const detail = { ...config, source };
    document.dispatchEvent(new CustomEvent("lightning__showtoast", { bubbles: true, composed: true, cancelable: true, detail }));
    return Promise.resolve(detail);
  }
`
	case "toastcontainer":
		classExtraJS = `  static instance() {
    return { maxToasts: 5, toastPosition: "top-center", containerPosition: "fixed" };
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
    if (this.__initialValue === undefined) {
      this.__initialValue = this.value;
    }
    this.reportUnsupportedAttributes();
    if (%[3]q === "lightning-input-field") {
      registerBaseFormFieldWithNearestForm(this, "__gladeInputFields");
    }
    if (%[3]q === "lightning-output-field") {
      registerBaseFormFieldWithNearestForm(this, "__gladeOutputFields");
    }
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
    if (event && event.stopPropagation) {
      event.stopPropagation();
    }
    const target = event && event.target || {};
    this.value = target.value;
    this.checked = Boolean(target.checked);
    this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value: target.value, checked: Boolean(target.checked) } }));
  }
  handleOptionGroupChange(event) {
    if (event && event.stopPropagation) {
      event.stopPropagation();
    }
    const target = event && event.target || {};
    if (%[3]q === "lightning-checkbox-group") {
      const values = Array.from(this.template && this.template.querySelectorAll ? this.template.querySelectorAll('input[type="checkbox"]:checked') : []).map((input) => input.value);
      this.value = values;
      this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value: values } }));
      return;
    }
    this.value = target.value;
    this.checked = Boolean(target.checked);
    this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value: target.value, checked: Boolean(target.checked) } }));
  }
  handleDualListboxMove(event) {
    if (event && event.stopPropagation) {
      event.stopPropagation();
    }
    const action = event && event.currentTarget && event.currentTarget.dataset && event.currentTarget.dataset.action || "";
    const current = selectedValueList(this.value);
    const sourceSelect = this.template && this.template.querySelector ? this.template.querySelector('[data-list="source"]') : null;
    const selectedSelect = this.template && this.template.querySelector ? this.template.querySelector('[data-list="selected"]') : null;
    const sourceValues = Array.from(sourceSelect && sourceSelect.selectedOptions || []).map((option) => option.value);
    const selectedValues = Array.from(selectedSelect && selectedSelect.selectedOptions || []).map((option) => option.value);
    let values = current.slice();
    if (action === "add") {
      for (const option of this.options || []) {
        const value = String(option.value ?? option.label ?? "");
        if (sourceValues.includes(value) && !values.includes(value)) {
          values.push(value);
        }
      }
    } else if (action === "remove") {
      const removing = new Set(selectedValues);
      values = values.filter((value) => !removing.has(value));
    } else if (action === "up") {
      const moving = new Set(selectedValues);
      for (let i = 1; i < values.length; i += 1) {
        if (moving.has(values[i]) && !moving.has(values[i - 1])) {
          [values[i - 1], values[i]] = [values[i], values[i - 1]];
        }
      }
    } else if (action === "down") {
      const moving = new Set(selectedValues);
      for (let i = values.length - 2; i >= 0; i -= 1) {
        if (moving.has(values[i]) && !moving.has(values[i + 1])) {
          [values[i], values[i + 1]] = [values[i + 1], values[i]];
        }
      }
    }
    this.value = values;
    this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value: values } }));
  }
  handleDualListboxChange(event) {
    if (event && event.stopPropagation) {
      event.stopPropagation();
    }
    const values = Array.from(event && event.target && event.target.selectedOptions || []).map((option) => option.value);
    this.value = values;
    this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value: values } }));
  }
  handleRichTextChange(event) {
    if (event && event.stopPropagation) {
      event.stopPropagation();
    }
    const target = event && event.target || {};
    const value = target.value ?? target.innerHTML ?? "";
    this.value = value;
    this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value } }));
  }
  handleFileUpload(event) {
    const files = Array.from(event && event.target && event.target.files || []);
    const uploadedFiles = files.map((file, index) => ({
      name: file.name,
      documentId: "069000000000" + String(index + 1).padStart(3, "0") + "AAA"
    }));
    this.dispatchEvent(new CustomEvent("uploadfinished", { bubbles: true, composed: true, detail: { files: uploadedFiles } }));
  }
  handleRecordPickerChange(event) {
    if (event && event.stopPropagation) {
      event.stopPropagation();
    }
    const target = event && event.target || {};
    this.value = target.value;
    this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { recordId: target.value, value: target.value } }));
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
  getErrors() {
    return this.__errors || null;
  }
  setErrors(errors) {
    this.__errors = errors || null;
  }
  getWiredData() {
    return this.__wiredData;
  }
  wireRecordUi(data) {
    this.__wiredData = data;
    applyBaseRecordUiToField(this, data);
  }
  getWiredPicklistValues() {
    return this.__wiredPicklistValues;
  }
  wirePicklistValues(data) {
    this.__wiredPicklistValues = data;
  }
  setValue(value) {
    this.value = value;
    this.dirty = true;
  }
  clean() {
    this.dirty = false;
  }
  reset() {
    this.value = this.__initialValue ?? "";
    this.dirty = false;
    this.__errors = null;
    this.__customValidityMessage = "";
    const control = baseFormControl(this);
    if (control) {
      if ("value" in control) {
        control.value = this.value ?? "";
      }
      if (control.setCustomValidity) {
        control.setCustomValidity("");
      }
    }
  }
  setCustomValidity(message) {
    this.__customValidityMessage = String(message || "");
    const control = baseFormControl(this);
    if (control && control.setCustomValidity) {
      control.setCustomValidity(this.__customValidityMessage);
    }
  }
  checkValidity() {
    const control = baseFormControl(this);
    if (control && control.setCustomValidity) {
      control.setCustomValidity(this.__customValidityMessage || "");
    }
    if (this.__customValidityMessage) {
      return false;
    }
    return control && control.checkValidity ? control.checkValidity() : true;
  }
  reportValidity() {
    const control = baseFormControl(this);
    if (control && control.setCustomValidity) {
      control.setCustomValidity(this.__customValidityMessage || "");
    }
    if (this.__customValidityMessage) {
      if (control && control.reportValidity) {
        control.reportValidity();
      }
      return false;
    }
    return control && control.reportValidity ? control.reportValidity() : true;
  }
  focus() {
    const control = baseFormControl(this);
    if (control && control.focus) {
      control.focus();
    }
  }
  blur() {
    const control = baseFormControl(this);
    if (control && control.blur) {
      control.blur();
    }
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
      applyBaseRecordUiToFormFields(this, {
        record: this.value,
        objectInfos: result && result.data && result.data.objectInfos || {},
        objectInfo: result && result.data && result.data.objectInfos && result.data.objectInfos[this.value && this.value.apiName]
      });
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
registerDecorators(%[2]s, { publicProps: basePublicProps(), publicMethods: basePublicMethods() });
function basePublicProps() {
  const props = {};
	for (const name of ["label","title","value","options","checked","disabled","type","variant","iconName","iconPosition","iconClass","alternativeText","size","columns","data","keyField","objectApiName","recordId","fields","mode","name","fieldName","error","content","href","target","street","city","province","postalCode","country","items","header","placeholder","accept","multiple","flowApiName","flowInputVariables","initials","fallbackIconName","labelWhenOff","labelWhenOn","labelWhenHover","selected","sourceLabel","selectedLabel","min","max","step","mapMarkers","zoomLevel","markersTitle","src","description","dirty","required","message","theme","defaultValue","latitude","longitude","salutation","firstName","middleName","lastName","suffix","informalName","format","formatStyle","displayValue","tabIndex","badgeCount","assistiveText","readOnly","maxToasts","toastPosition","containerPosition","expanded","horizontalAlign","verticalAlign","pullToBoundary","multipleRows","smallDeviceSize","mediumDeviceSize","largeDeviceSize","padding","flexibility","alignmentBump","currencyCode","currencyDisplayAs","minimumIntegerDigits","minimumFractionDigits","maximumFractionDigits","minimumSignificantDigits","maximumSignificantDigits"]) {
    props[name] = { config: 0 };
  }
  return props;
}
function basePublicMethods() {
  return ["setErrors","getErrors","wireRecordUi","getWiredData","wirePicklistValues","getWiredPicklistValues","setValue","clean","reset","setCustomValidity","checkValidity","reportValidity","focus","blur"];
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
function baseFormControl(component) {
  const template = component && component.template;
  if (!template || !template.querySelector) {
    return null;
  }
  return template.querySelector("input, textarea, select, button, [tabindex]");
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
function applyBaseRecordUiToFormFields(form, data) {
  for (const field of baseFormFieldComponents(form)) {
    if (field && field.wireRecordUi) {
      field.wireRecordUi(data);
    }
  }
}
function applyBaseRecordUiToField(field, data) {
  const name = fieldApiName(field && (field.fieldName || field.name));
  if (!name) {
    return;
  }
  const record = data && data.record || {};
  const objectInfo = data && (data.objectInfo || data.objectInfos && data.objectInfos[record && record.apiName]) || {};
  const metadata = objectInfo && objectInfo.fields && objectInfo.fields[name] || {};
  const recordField = record && record.fields && record.fields[name];
  const value = recordField && typeof recordField === "object" ? recordField.value : recordField;
  if (value !== undefined) {
    field.value = value;
    field.__initialValue = value;
    field.dirty = false;
  }
  if (!field.label && metadata.label) {
    field.label = metadata.label;
  }
  if (field.required === undefined && metadata.required !== undefined) {
    field.required = Boolean(metadata.required);
  }
  if (!field.type) {
    const inputType = baseInputTypeForMetadata(metadata);
    if (inputType) {
      field.type = inputType;
    }
  }
  if (!field.options && Array.isArray(metadata.picklistValues)) {
    field.options = metadata.picklistValues.map((option) => ({ label: option.label ?? option.value ?? "", value: option.value ?? option.label ?? "" }));
  }
}
function baseFormFieldComponents(root) {
  const fields = [
    ...(root && root.hostElement && root.hostElement.__gladeInputFields || []),
    ...(root && root.template && root.template.host && root.template.host.__gladeInputFields || []),
    ...(root && root.hostElement && root.hostElement.__gladeOutputFields || []),
    ...(root && root.template && root.template.host && root.template.host.__gladeOutputFields || [])
  ];
  for (const selector of ["lightning-input-field", "lightning-output-field"]) {
    fields.push(...(root && root.querySelectorAll ? root.querySelectorAll(selector) : []));
    for (const container of [root, root && root.hostElement, root && root.template && root.template.host]) {
      for (const element of container && container.children || []) collectBaseAssignedFields(element, fields, selector);
    }
    for (const slot of root && root.template && root.template.querySelectorAll ? root.template.querySelectorAll("slot") : []) {
      for (const element of slot.assignedElements ? slot.assignedElements({ flatten: true }) : []) collectBaseAssignedFields(element, fields, selector);
    }
  }
  return [...new Set(fields)];
}
function registerBaseFormFieldWithNearestForm(component, propertyName) {
  const host = component && (component.hostElement || component.template && component.template.host);
  let node = host && host.parentElement || null;
  while (node) {
    const tag = node.tagName && node.tagName.toLowerCase();
    if (tag === "lightning-record-edit-form" || tag === "lightning-record-form" || tag === "lightning-record-view-form") {
      node[propertyName] = node[propertyName] || [];
      if (!node[propertyName].includes(component)) node[propertyName].push(component);
      return;
    }
    const root = node.getRootNode && node.getRootNode();
    node = node.parentElement || root && root.host && root.host.parentElement || null;
  }
}
function collectBaseAssignedFields(element, fields, selector) {
  if (!element) {
    return;
  }
  if (element.tagName && element.tagName.toLowerCase() === selector) {
    fields.push(element);
  }
  for (const child of element.querySelectorAll ? element.querySelectorAll(selector) : []) {
    fields.push(child);
  }
}
function baseInputTypeForMetadata(metadata) {
  const type = String(metadata && (metadata.dataType || metadata.type) || "").toLowerCase();
  if (["picklist", "multipicklist"].includes(type)) return "";
  if (["boolean", "checkbox"].includes(type)) return "checkbox";
  if (["double", "integer", "currency", "percent", "number"].includes(type)) return "number";
  if (type === "date") return "date";
  if (["datetime", "datetime-local"].includes(type)) return "datetime-local";
  if (type === "email") return "email";
  if (["phone", "tel"].includes(type)) return "tel";
  return "";
}
function selectedValueList(value) {
  if (Array.isArray(value)) {
    return value.map(String);
  }
  if (typeof value === "string" && value.trim()) {
    return value.split(",").map((item) => item.trim()).filter(Boolean);
  }
  if (value == null) {
    return [];
  }
  return [String(value)];
}
function flattenTreeRows(rows, level = 0) {
  const out = [];
  for (const row of rows || []) {
    out.push({ row, level });
    out.push(...flattenTreeRows(row._children || row.children || row.items || [], level + 1));
  }
  return out;
}
function markerText(marker) {
  const location = marker && marker.location || {};
  return [
    marker && marker.title,
    marker && marker.value,
    location.Name,
    location.City,
    location.State,
    location.Country,
    location.Street
  ].filter(Boolean).join(", ");
}
function normalizeChoice(value, validValues, fallbackValue) {
  const text = String(value ?? "").toLowerCase();
  return validValues.includes(text) ? text : fallbackValue;
}
function normalizedButtonType(type) {
  return normalizeChoice(type, ["button", "reset", "submit"], "button");
}
function buttonClassMap(variant) {
  const normalized = normalizeChoice(variant, ["base", "neutral", "brand", "destructive", "inverse", "success"], "neutral");
  return {
    "slds-button": true,
    "slds-button_neutral": normalized === "neutral",
    "slds-button_brand": normalized === "brand",
    "slds-button_destructive": normalized === "destructive",
    "slds-button_inverse": normalized === "inverse",
    "slds-button_success": normalized === "success"
  };
}
function buttonIconClassMap(variant, size) {
  const normalizedVariant = normalizeChoice(variant, ["bare", "brand", "container", "border", "border-filled", "bare-inverse", "border-inverse"], "border");
  const normalizedSize = normalizeChoice(size, ["xx-small", "x-small", "small", "medium", "large"], "medium");
  const isBare = normalizedVariant.startsWith("bare");
  return {
    "slds-button": true,
    "slds-button_icon": true,
    "slds-button_icon-bare": isBare,
    "slds-button_icon-container": normalizedVariant === "container",
    "slds-button_icon-border": normalizedVariant === "border",
    "slds-button_icon-border-filled": normalizedVariant === "border-filled",
    "slds-button_icon-border-inverse": normalizedVariant === "border-inverse",
    "slds-button_icon-inverse": normalizedVariant === "bare-inverse",
    "slds-button_icon-brand": normalizedVariant === "brand",
    "slds-button_icon-small": !isBare && normalizedSize === "small",
    "slds-button_icon-x-small": !isBare && normalizedSize === "x-small",
    "slds-button_icon-xx-small": !isBare && normalizedSize === "xx-small"
  };
}
function cardClassMap(variant) {
  return {
    "slds-card": true,
    "slds-card_narrow": normalizeChoice(variant, ["base", "narrow"], "base") === "narrow"
  };
}
function layoutClassMap(component) {
  const horizontal = {
    center: "slds-grid_align-center",
    space: "slds-grid_align-space",
    spread: "slds-grid_align-spread",
    end: "slds-grid_align-end"
  };
  const vertical = {
    start: "slds-grid_vertical-align-start",
    center: "slds-grid_vertical-align-center",
    end: "slds-grid_vertical-align-end",
    stretch: "slds-grid_vertical-stretch"
  };
  const boundary = {
    small: "slds-grid_pull-padded",
    medium: "slds-grid_pull-padded-medium",
    large: "slds-grid_pull-padded-large"
  };
  const classes = { "slds-grid": true };
  const hClass = horizontal[normalizeChoice(component && component.horizontalAlign, Object.keys(horizontal), "")];
  const vClass = vertical[normalizeChoice(component && component.verticalAlign, Object.keys(vertical), "")];
  const bClass = boundary[normalizeChoice(component && component.pullToBoundary, Object.keys(boundary), "")];
  if (hClass) classes[hClass] = true;
  if (vClass) classes[vClass] = true;
  if (bClass) classes[bClass] = true;
  if (Boolean(component && component.multipleRows)) classes["slds-wrap"] = true;
  return classes;
}
function normalizedLayoutSize(value) {
  if (value === undefined || value === null || value === "") {
    return null;
  }
  const size = Math.round(Number(value));
  return Number.isFinite(size) && size >= 1 && size <= 12 ? size : null;
}
function layoutItemClassMap(component) {
  const classes = { "slds-col": true };
  const padding = String(component && component.padding || "").toLowerCase();
  const paddingClasses = {
    "horizontal-small": ["slds-p-right_small", "slds-p-left_small"],
    "horizontal-medium": ["slds-p-right_medium", "slds-p-left_medium"],
    "horizontal-large": ["slds-p-right_large", "slds-p-left_large"],
    "around-small": ["slds-p-around_small"],
    "around-medium": ["slds-p-around_medium"],
    "around-large": ["slds-p-around_large"]
  };
  for (const className of paddingClasses[padding] || []) classes[className] = true;
  const flexValues = Array.isArray(component && component.flexibility) ? component.flexibility : String(component && component.flexibility || "").split(",").map((item) => item.trim()).filter(Boolean);
  const flexClasses = {
    auto: "slds-col",
    grow: "slds-grow",
    shrink: "slds-shrink",
    "no-grow": "slds-grow-none",
    "no-shrink": "slds-shrink-none",
    "no-flex": "slds-no-flex"
  };
  for (const value of flexValues) {
    if (flexClasses[value]) classes[flexClasses[value]] = true;
  }
  for (const [prop, prefix] of [["size", "slds-size_"], ["smallDeviceSize", "slds-small-size_"], ["mediumDeviceSize", "slds-medium-size_"], ["largeDeviceSize", "slds-large-size_"]]) {
    const size = normalizedLayoutSize(component && component[prop]);
    if (size) classes[prefix + size + "-of-12"] = true;
  }
  const bump = normalizeChoice(component && component.alignmentBump, ["left", "top", "right", "bottom"], "");
  if (bump) classes["slds-col_bump-" + bump] = true;
  return classes;
}
function numericOption(value) {
  if (value === undefined || value === null || value === "") {
    return undefined;
  }
  const number = Number(value);
  return Number.isFinite(number) ? number : undefined;
}
function formatNumberValue(component) {
  const raw = component && component.value;
  if (raw === undefined || raw === null || raw === "" || !Number.isFinite(Number(raw))) {
    return "";
  }
  let style = normalizeChoice(component && component.formatStyle, ["decimal", "currency", "percent", "percent-fixed"], "decimal");
  let value = Number(raw);
  if (style === "percent-fixed") {
    style = "percent";
    value = value / 100;
  }
  const options = { style };
  if (style === "currency") {
    options.currency = component && component.currencyCode || "USD";
    options.currencyDisplay = normalizeChoice(component && component.currencyDisplayAs, ["symbol", "code", "name"], "symbol");
  }
  for (const [prop, option] of [
    ["minimumIntegerDigits", "minimumIntegerDigits"],
    ["minimumFractionDigits", "minimumFractionDigits"],
    ["maximumFractionDigits", "maximumFractionDigits"],
    ["minimumSignificantDigits", "minimumSignificantDigits"],
    ["maximumSignificantDigits", "maximumSignificantDigits"]
  ]) {
    const parsed = numericOption(component && component[prop]);
    if (parsed !== undefined) options[option] = parsed;
  }
  try {
    return new Intl.NumberFormat(undefined, options).format(value);
  } catch (_err) {
    return String(raw);
  }
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
	return nil
}

func buttonTemplateJS() string {
	return `[api_element("button", { classMap: buttonClassMap($cmp.variant), attrs: { type: normalizedButtonType($cmp.type), name: $cmp.name || undefined, value: $cmp.value == null ? undefined : String($cmp.value), title: $cmp.title || undefined, "aria-label": $cmp.alternativeText || undefined }, props: { disabled: Boolean($cmp.disabled) }, key: 0 }, [api_text($cmp.iconName && $cmp.iconPosition !== "right" ? ($cmp.iconName || "").split(":").pop() + " " : ""), api_text($cmp.label || ""), api_text($cmp.iconName && $cmp.iconPosition === "right" ? " " + ($cmp.iconName || "").split(":").pop() : "")])]`
}

func buttonStatefulTemplateJS() string {
	return `[api_element("button", { classMap: { ...buttonClassMap($cmp.variant), "slds-button_stateful": true, "slds-is-selected": Boolean($cmp.selected || $cmp.checked), "slds-not-selected": !Boolean($cmp.selected || $cmp.checked) }, attrs: { type: "button", "aria-pressed": String(Boolean($cmp.selected || $cmp.checked)) }, props: { disabled: Boolean($cmp.disabled) }, key: 0 }, [api_text($cmp.label || ((($cmp.selected || $cmp.checked) ? $cmp.labelWhenOn : $cmp.labelWhenOff) || $cmp.labelWhenHover || ""))])]`
}

func iconButtonTemplateJS() string {
	return `[api_element("button", { classMap: buttonIconClassMap($cmp.variant, $cmp.size), attrs: { type: normalizedButtonType($cmp.type), name: $cmp.name || undefined, value: $cmp.value == null ? undefined : String($cmp.value), title: $cmp.alternativeText || $cmp.iconName || "", "aria-label": $cmp.alternativeText || undefined }, props: { disabled: Boolean($cmp.disabled) }, key: 0 }, [api_text(($cmp.iconName || "utility:button").split(":").pop()), $cmp.alternativeText ? api_element("span", { classMap: { "slds-assistive-text": true }, key: 1 }, [api_text($cmp.alternativeText)]) : null].filter(Boolean))]`
}

func cardTemplateJS() string {
	return `(() => {
  const titleChildren = $cmp.title ? [api_text($cmp.title)] : [api_slot("title", { key: 8 }, [], $slotset)];
  const mediaChildren = [];
  if ($cmp.iconName) {
    mediaChildren.push(api_element("span", { classMap: { "slds-media__figure": true, "slds-icon_container": true }, attrs: { title: $cmp.iconName }, key: 4 }, [api_text(($cmp.iconName || "").split(":").pop())]));
  }
  mediaChildren.push(api_element("div", { classMap: { "slds-media__body": true, "slds-truncate": true }, key: 5 }, [api_element("h2", { classMap: { "slds-card__header-title": true }, key: 6 }, [api_element("span", { classMap: { "slds-text-heading_small": true }, key: 7 }, titleChildren)])]));
  return [api_element("article", { classMap: cardClassMap($cmp.variant), key: 0 }, [
    api_element("header", { classMap: { "slds-card__header": true, "slds-grid": true }, key: 1 }, [
      api_element("div", { classMap: { "slds-media": true, "slds-media_center": true, "slds-has-flexi-truncate": true }, key: 2 }, mediaChildren),
      api_element("div", { classMap: { "slds-no-flex": true }, key: 9 }, [api_slot("actions", { key: 10 }, [], $slotset)])
    ]),
    api_element("div", { classMap: { "slds-card__body": true }, key: 11 }, [api_slot("", { key: 12 }, [], $slotset)]),
    api_element("div", { classMap: { "slds-card__footer": true }, key: 13 }, [api_slot("footer", { key: 14 }, [], $slotset)])
  ])];
})()`
}

func inputTemplateJS() string {
	return `[api_element("label", { classMap: { "slds-form-element": true }, key: 0 }, [api_element("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [api_text($cmp.label || "")]), api_element("input", { classMap: { "slds-input": true }, attrs: { type: $cmp.type || "text" }, props: { value: $cmp.value || "", disabled: Boolean($cmp.disabled), required: Boolean($cmp.required) }, key: 2, on: { change: api_bind($cmp.handleChange), input: api_bind($cmp.handleChange) } })])]`
}

func textareaTemplateJS() string {
	return `[api_element("label", { classMap: { "slds-form-element": true }, key: 0 }, [api_element("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [api_text($cmp.label || "")]), api_element("textarea", { classMap: { "slds-textarea": true }, props: { value: $cmp.value || "", required: Boolean($cmp.required) }, key: 2, on: { change: api_bind($cmp.handleChange), input: api_bind($cmp.handleChange) } })])]`
}

func comboboxTemplateJS() string {
	return `[api_element("label", { classMap: { "slds-form-element": true, "slds-combobox": true }, key: 0 }, [api_element("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [api_text($cmp.label || "")]), api_element("select", { classMap: { "slds-select": true }, props: { value: $cmp.value || "", required: Boolean($cmp.required) }, key: 2, on: { change: api_bind($cmp.handleChange) } }, ($cmp.options || []).map((option, index) => api_element("option", { attrs: { value: String(option.value ?? option.label ?? "") }, props: { selected: String(option.value ?? option.label ?? "") === String($cmp.value ?? "") }, key: 20 + index }, [api_text(option.label || option.value || "")])))])]`
}

func layoutTemplateJS() string {
	return `[api_element("div", { classMap: layoutClassMap($cmp), key: 0 }, [api_slot("", { key: 1 }, [], $slotset)])]`
}

func layoutItemTemplateJS() string {
	return `[api_element("div", { classMap: layoutItemClassMap($cmp), key: 0 }, [api_slot("", { key: 1 }, [], $slotset)])]`
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
	return `[api_element("section", { classMap: { "slds-box": true, "slds-theme_default": true }, attrs: { role: "region" }, key: 0 }, [api_element("header", { classMap: { "slds-modal__header": true }, key: 1 }, [api_text($cmp.label || $cmp.title || "")]), api_element("div", { classMap: { "slds-modal__content": true }, key: 2 }, [api_slot("", { key: 3 }, [], $slotset)])])]`
}

func textContainerTemplateJS(tag, className string) string {
	return fmt.Sprintf(`[api_element(%q, { classMap: { %q: true }, attrs: { title: String($cmp.value ?? $cmp.label ?? "") }, key: 0 }, [api_text($cmp.value ?? $cmp.label ?? "")])]`, tag, className)
}

func formattedEmailTemplateJS() string {
	return `[api_element("a", { attrs: { href: ($cmp.value || $cmp.href) ? "mailto:" + String($cmp.value || $cmp.href) : "#" }, key: 0 }, [api_text($cmp.label || $cmp.value || $cmp.href || "")])]`
}

func formattedNumberTemplateJS() string {
	return `(() => { const text = formatNumberValue($cmp); return [api_element("span", { classMap: { "slds-truncate": true }, attrs: { title: text }, key: 0 }, [api_text(text)])]; })()`
}

func slotContainerTemplateJS(tag, className string) string {
	return fmt.Sprintf(`[api_element(%q, { classMap: { %q: true }, key: 0 }, [api_slot("", { key: 1 }, [], $slotset)])]`, tag, className)
}

func breadcrumbsTemplateJS() string {
	return `[api_element("nav", { classMap: { "slds-breadcrumb": true }, attrs: { role: "navigation" }, key: 0 }, [api_element("ol", { classMap: { "slds-breadcrumb__list": true }, key: 1 }, [api_slot("", { key: 2 }, [], $slotset)])])]`
}

func breadcrumbTemplateJS() string {
	return `[api_element("a", { classMap: { "slds-breadcrumb__item": true }, attrs: { href: $cmp.href || "#" }, key: 0, on: { click: api_bind($cmp.handleActive) } }, [api_text($cmp.label || $cmp.name || "")])]`
}

func titledSlotTemplateJS(tag, className string) string {
	return fmt.Sprintf(`[api_element(%q, { classMap: { %q: true }, key: 0 }, [api_element("h3", { key: 1 }, [api_text($cmp.label || $cmp.name || "")]), api_slot("", { key: 2 }, [], $slotset)])]`, tag, className)
}

func formattedLinkTemplateJS(kind string) string {
	if kind == "tel" {
		return `[api_element("a", { attrs: { href: "tel:" + String($cmp.value || "") }, key: 0 }, [api_text($cmp.value || "")])]`
	}
	return `[api_element("a", { attrs: { href: $cmp.value || $cmp.href || "#", target: $cmp.target || undefined }, key: 0 }, [api_text($cmp.label || $cmp.value || $cmp.href || "")])]`
}

func formattedAddressTemplateJS() string {
	return `[api_element("address", { classMap: { "slds-truncate": true }, key: 0 }, [api_text([$cmp.street, $cmp.city, $cmp.province, $cmp.postalCode, $cmp.country, $cmp.value].filter(Boolean).join(", "))])]`
}

func formattedLocationTemplateJS() string {
	return `[api_element("span", { classMap: { "slds-truncate": true }, key: 0 }, [api_text([$cmp.latitude, $cmp.longitude].filter((value) => value !== undefined && value !== null && value !== "").join(", "))])]`
}

func formattedNameTemplateJS() string {
	return `[api_element("span", { classMap: { "slds-truncate": true }, key: 0 }, [api_text([$cmp.salutation, $cmp.firstName, $cmp.middleName, $cmp.lastName, $cmp.suffix, $cmp.informalName, $cmp.value].filter(Boolean).join(" "))])]`
}

func formattedLookupTemplateJS() string {
	return `[api_element("a", { attrs: { href: $cmp.href || ($cmp.recordId ? "/lightning/r/" + ($cmp.objectApiName || "Record") + "/" + $cmp.recordId + "/view" : "#"), tabindex: $cmp.tabIndex == null ? undefined : String($cmp.tabIndex) }, key: 0, on: { click: api_bind($cmp.handleActive) } }, [api_text($cmp.displayValue || $cmp.label || $cmp.recordId || "")])]`
}

func inputLocationTemplateJS() string {
	return `[api_element("fieldset", { classMap: { "slds-form-element": true }, key: 0 }, [api_element("legend", { classMap: { "slds-form-element__legend": true }, key: 1 }, [api_text($cmp.label || "Location")]), api_element("input", { classMap: { "slds-input": true }, attrs: { type: "number", step: "any", placeholder: "Latitude" }, props: { value: $cmp.latitude ?? "", disabled: Boolean($cmp.disabled), required: Boolean($cmp.required) }, key: 2, on: { change: api_bind($cmp.handleChange), input: api_bind($cmp.handleChange) } }), api_element("input", { classMap: { "slds-input": true }, attrs: { type: "number", step: "any", placeholder: "Longitude" }, props: { value: $cmp.longitude ?? "", disabled: Boolean($cmp.disabled), required: Boolean($cmp.required) }, key: 3, on: { change: api_bind($cmp.handleChange), input: api_bind($cmp.handleChange) } })])]`
}

func inputNameTemplateJS() string {
	return `[api_element("fieldset", { classMap: { "slds-form-element": true }, key: 0 }, [api_element("legend", { classMap: { "slds-form-element__legend": true }, key: 1 }, [api_text($cmp.label || "Name")]), api_element("input", { classMap: { "slds-input": true }, attrs: { placeholder: "First Name" }, props: { value: $cmp.firstName || "", disabled: Boolean($cmp.disabled), required: Boolean($cmp.required) }, key: 2, on: { change: api_bind($cmp.handleChange), input: api_bind($cmp.handleChange) } }), api_element("input", { classMap: { "slds-input": true }, attrs: { placeholder: "Last Name" }, props: { value: $cmp.lastName || "", disabled: Boolean($cmp.disabled), required: Boolean($cmp.required) }, key: 3, on: { change: api_bind($cmp.handleChange), input: api_bind($cmp.handleChange) } })])]`
}

func dialogNoticeTemplateJS(kind string) string {
	return fmt.Sprintf(`[api_element("section", { classMap: { "slds-modal": true, "slds-fade-in-open": true }, attrs: { role: %[1]q }, key: 0 }, [api_element("div", { classMap: { "slds-modal__container": true }, key: 1 }, [api_element("header", { classMap: { "slds-modal__header": true }, key: 2 }, [api_text($cmp.label || $cmp.title || %[1]q)]), api_element("div", { classMap: { "slds-modal__content": true }, key: 3 }, [api_text($cmp.message || $cmp.value || ""), api_slot("", { key: 4 }, [], $slotset)])])])]`, kind)
}

func modalHeaderTemplateJS() string {
	return `[api_element("header", { classMap: { "slds-modal__header": true }, key: 0 }, [api_element("h2", { classMap: { "slds-modal__title": true }, key: 1 }, [api_text($cmp.label || $cmp.title || "")]), api_slot("", { key: 2 }, [], $slotset)])]`
}

func dynamicIconTemplateJS() string {
	return `[api_element("span", { classMap: { "slds-icon_container": true }, attrs: { title: $cmp.alternativeText || $cmp.type || $cmp.iconName || "" }, key: 0 }, [api_text($cmp.alternativeText || $cmp.type || $cmp.iconName || "")])]`
}

func barcodeScannerTemplateJS() string {
	return `[api_element("button", { classMap: { "slds-button": true, "slds-button_neutral": true }, attrs: { type: "button" }, props: { disabled: Boolean($cmp.disabled) }, key: 0 }, [api_text($cmp.label || "Scan Barcode")])]`
}

func primitiveFigureTemplateJS() string {
	return `[api_element("figure", { classMap: { "slds-figure": true }, key: 0 }, [api_slot("", { key: 1 }, [], $slotset), api_element("figcaption", { key: 2 }, [api_text($cmp.label || $cmp.title || "")])])]`
}

func relativeDateTimeTemplateJS() string {
	return `[api_element("time", { classMap: { "slds-truncate": true }, attrs: { datetime: String($cmp.value || "") }, key: 0 }, [api_text($cmp.value || "")])]`
}

func stackedTabTemplateJS() string {
	return `[api_element("button", { classMap: { "slds-button": true, "slds-button_neutral": true }, attrs: { type: "button" }, props: { disabled: Boolean($cmp.disabled) }, key: 0, on: { click: api_bind($cmp.handleActive) } }, [api_text($cmp.label || $cmp.name || "")])]`
}

func toastTemplateJS() string {
	return `[api_element("section", { classMap: { "slds-notify": true, "slds-notify_toast": true }, attrs: { role: ($cmp.variant === "error" ? "alert" : "status") }, key: 0 }, [api_element("h2", { classMap: { "slds-text-heading_small": true }, key: 1 }, [api_text($cmp.label || $cmp.title || "")]), api_element("div", { classMap: { "slds-notify__content": true }, key: 2 }, [api_text($cmp.message || $cmp.value || ""), api_slot("", { key: 3 }, [], $slotset)])])]`
}

func avatarTemplateJS() string {
	return `[api_element("span", { classMap: { "slds-avatar": true }, attrs: { title: $cmp.alternativeText || $cmp.label || "" }, key: 0 }, [api_text(($cmp.initials || $cmp.fallbackIconName || $cmp.label || "avatar").toString())])]`
}

func helptextTemplateJS() string {
	return `[api_element("span", { classMap: { "slds-form-element__icon": true }, attrs: { title: $cmp.content || $cmp.label || "" }, key: 0 }, [api_text($cmp.content || $cmp.label || "?")])]`
}

func buttonMenuTemplateJS() string {
	return `[api_element("div", { classMap: { "slds-dropdown-trigger": true, "slds-dropdown-trigger_click": true }, key: 0 }, [api_element("button", { classMap: { "slds-button": true, "slds-button_neutral": true }, attrs: { type: "button" }, key: 1 }, [api_text($cmp.label || "Actions")]), api_element("div", { classMap: { "slds-dropdown": true }, key: 2 }, [api_slot("", { key: 3 }, [], $slotset)])])]`
}

func menuItemTemplateJS() string {
	return `[api_element("button", { classMap: { "slds-dropdown__item": true }, attrs: { type: "button", role: "menuitem" }, key: 0, on: { click: api_bind($cmp.handleActive) } }, [api_text($cmp.label || $cmp.value || "")])]`
}

func optionGroupTemplateJS(inputType string) string {
	return fmt.Sprintf(`[api_element("fieldset", { classMap: { "slds-form-element": true }, key: 0 }, [api_element("legend", { classMap: { "slds-form-element__legend": true }, key: 1 }, [api_text($cmp.label || "")]), api_element("div", { classMap: { "slds-form-element__control": true }, key: 2 }, ($cmp.options || []).map((option, index) => { const optionValue = String(option.value ?? option.label ?? ""); return api_element("label", { classMap: { "slds-%[1]s": true }, key: 20 + index }, [api_element("input", { attrs: { type: %[1]q, value: optionValue, name: $cmp.name || $cmp.label || %[1]q }, props: { checked: selectedValueList($cmp.value).includes(optionValue) }, key: 200 + index, on: { change: api_bind($cmp.handleOptionGroupChange) } }), api_element("span", { key: 400 + index }, [api_text(option.label || option.value || "")])]); }) )])]`, inputType)
}

func selectTemplateJS() string {
	return `[api_element("label", { classMap: { "slds-form-element": true }, key: 0 }, [api_element("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [api_text($cmp.label || "")]), api_element("select", { classMap: { "slds-select": true }, props: { value: String($cmp.value ?? ""), disabled: Boolean($cmp.disabled) }, key: 2, on: { change: api_bind($cmp.handleChange) } }, ($cmp.options || []).map((option, index) => { const optionValue = String(option.value ?? option.label ?? ""); return api_element("option", { attrs: { value: optionValue }, props: { selected: optionValue === String($cmp.value ?? "") }, key: 20 + index }, [api_text(option.label || option.value || "")]); }))])]`
}

func sliderTemplateJS() string {
	return `[api_element("label", { classMap: { "slds-form-element": true, "slds-slider": true }, key: 0 }, [api_element("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [api_text($cmp.label || "")]), api_element("input", { classMap: { "slds-slider__range": true }, attrs: { type: "range", min: String($cmp.min ?? "0"), max: String($cmp.max ?? "100"), step: String($cmp.step ?? "1") }, props: { value: String($cmp.value ?? "0"), disabled: Boolean($cmp.disabled) }, key: 2, on: { change: api_bind($cmp.handleChange), input: api_bind($cmp.handleChange) } })])]`
}

func dualListboxTemplateJS() string {
	return `(() => {
  const values = selectedValueList($cmp.value);
  const options = $cmp.options || [];
  const sourceOptions = options.filter((option) => !values.includes(String(option.value ?? option.label ?? "")));
  const selectedOptions = values.map((value) => options.find((option) => String(option.value ?? option.label ?? "") === value) || { label: value, value });
  return [api_element("fieldset", { classMap: { "slds-form-element": true, "slds-dueling-list": true }, key: 0 }, [
    api_element("legend", { classMap: { "slds-form-element__legend": true }, key: 1 }, [api_text($cmp.label || "")]),
    api_element("div", { classMap: { "slds-dueling-list__column": true }, key: 2 }, [
      api_element("span", { key: 3 }, [api_text($cmp.sourceLabel || "Available")]),
      api_element("select", { classMap: { "slds-select": true }, attrs: { multiple: "", "data-list": "source" }, props: { disabled: Boolean($cmp.disabled) }, key: 4 }, sourceOptions.map((option, index) => {
        const optionValue = String(option.value ?? option.label ?? "");
        return api_element("option", { attrs: { value: optionValue }, key: 40 + index }, [api_text(option.label || option.value || "")]);
      }))
    ]),
    api_element("div", { classMap: { "slds-dueling-list__column": true }, key: 5 }, [
      api_element("button", { classMap: { "slds-button": true, "slds-button_icon": true }, attrs: { type: "button", "data-action": "add", "aria-label": "Move selection to Selected" }, props: { disabled: Boolean($cmp.disabled) }, key: 50, on: { click: api_bind($cmp.handleDualListboxMove) } }, [api_text("Move selection to Selected")]),
      api_element("button", { classMap: { "slds-button": true, "slds-button_icon": true }, attrs: { type: "button", "data-action": "remove", "aria-label": "Move selection to Available" }, props: { disabled: Boolean($cmp.disabled) }, key: 51, on: { click: api_bind($cmp.handleDualListboxMove) } }, [api_text("Move selection to Available")]),
      api_element("button", { classMap: { "slds-button": true, "slds-button_icon": true }, attrs: { type: "button", "data-action": "up", "aria-label": "Move selection up" }, props: { disabled: Boolean($cmp.disabled) }, key: 52, on: { click: api_bind($cmp.handleDualListboxMove) } }, [api_text("Move selection up")]),
      api_element("button", { classMap: { "slds-button": true, "slds-button_icon": true }, attrs: { type: "button", "data-action": "down", "aria-label": "Move selection down" }, props: { disabled: Boolean($cmp.disabled) }, key: 53, on: { click: api_bind($cmp.handleDualListboxMove) } }, [api_text("Move selection down")])
    ]),
    api_element("div", { classMap: { "slds-dueling-list__column": true }, key: 6 }, [
      api_element("span", { key: 7 }, [api_text($cmp.selectedLabel || "Selected")]),
      api_element("select", { classMap: { "slds-select": true }, attrs: { multiple: "", "data-list": "selected" }, props: { disabled: Boolean($cmp.disabled) }, key: 8 }, selectedOptions.map((option, index) => {
        const optionValue = String(option.value ?? option.label ?? "");
        return api_element("option", { attrs: { value: optionValue }, key: 80 + index }, [api_text(option.label || option.value || "")]);
      }))
    ])
  ])];
})()`
}

func inputRichTextTemplateJS() string {
	return `[api_element("label", { classMap: { "slds-form-element": true }, key: 0 }, [api_element("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [api_text($cmp.label || "")]), api_element("textarea", { classMap: { "slds-textarea": true, "slds-rich-text-editor__textarea": true }, props: { value: $cmp.value || "", disabled: Boolean($cmp.disabled) }, key: 2, on: { change: api_bind($cmp.handleRichTextChange), input: api_bind($cmp.handleRichTextChange) } })])]`
}

func inputAddressTemplateJS() string {
	return `[api_element("fieldset", { classMap: { "slds-form-element": true }, key: 0 }, [api_element("legend", { classMap: { "slds-form-element__legend": true }, key: 1 }, [api_text($cmp.label || "Address")]), api_element("input", { classMap: { "slds-input": true }, attrs: { placeholder: "Street" }, props: { value: $cmp.street || "" }, key: 2, on: { change: api_bind($cmp.handleChange) } }), api_element("input", { classMap: { "slds-input": true }, attrs: { placeholder: "City" }, props: { value: $cmp.city || "" }, key: 3, on: { change: api_bind($cmp.handleChange) } })])]`
}

func fileUploadTemplateJS() string {
	return `[api_element("label", { classMap: { "slds-form-element": true }, key: 0 }, [api_element("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [api_text($cmp.label || "Upload Files")]), api_element("input", { classMap: { "slds-file-selector__input": true }, attrs: { type: "file", accept: $cmp.accept || undefined, multiple: $cmp.multiple ? "" : undefined }, props: { disabled: Boolean($cmp.disabled) }, key: 2, on: { change: api_bind($cmp.handleFileUpload) } })])]`
}

func flowTemplateJS() string {
	return `[api_element("section", { classMap: { "slds-box": true }, attrs: { "data-flow-api-name": $cmp.flowApiName || "" }, key: 0 }, [api_text($cmp.flowApiName || $cmp.label || "Flow")])]`
}

func pillTemplateJS() string {
	return `[api_element("span", { classMap: { "slds-pill": true }, key: 0 }, [api_text($cmp.label || $cmp.name || "")])]`
}

func pillContainerTemplateJS() string {
	return `[api_element("div", { classMap: { "slds-pill_container": true }, key: 0 }, ($cmp.items || []).map((item, index) => api_element("span", { classMap: { "slds-pill": true }, key: 20 + index }, [api_text(item.label || item.name || item.value || "")])).concat([api_slot("", { key: 1 }, [], $slotset)]))]`
}

func menuDividerTemplateJS() string {
	return `[api_element("div", { classMap: { "slds-has-divider_top-space": true }, attrs: { role: "separator" }, key: 0 })]`
}

func progressBarTemplateJS() string {
	return `(() => { const value = String($cmp.value ?? "0"); return [api_element("div", { classMap: { "slds-progress-bar": true }, attrs: { role: "progressbar", "aria-valuemin": "0", "aria-valuemax": "100", "aria-valuenow": value }, key: 0 }, [api_element("span", { classMap: { "slds-progress-bar__value": true }, key: 1 }, [api_text(value + "%")])])]; })()`
}

func progressRingTemplateJS() string {
	return `(() => { const value = String($cmp.value ?? "0"); return [api_element("div", { classMap: { "slds-progress-ring": true }, attrs: { role: "progressbar", "aria-valuenow": value }, key: 0 }, [api_text(value + "%")])]; })()`
}

func tileTemplateJS() string {
	return `[api_element("article", { classMap: { "slds-tile": true }, key: 0 }, [api_element("h3", { classMap: { "slds-tile__title": true }, key: 1 }, [api_element("a", { attrs: { href: $cmp.href || "#" }, key: 2 }, [api_text($cmp.label || $cmp.title || "")])]), api_element("div", { classMap: { "slds-tile__detail": true }, key: 3 }, [api_slot("", { key: 4 }, [], $slotset)])])]`
}

func quickActionPanelTemplateJS() string {
	return `[api_element("section", { classMap: { "slds-box": true, "slds-theme_default": true }, attrs: { role: "region" }, key: 0 }, [api_element("header", { classMap: { "slds-modal__header": true }, key: 1 }, [api_text($cmp.header || $cmp.title || "")]), api_element("div", { classMap: { "slds-modal__content": true }, key: 2 }, [api_slot("", { key: 3 }, [], $slotset)]), api_element("footer", { classMap: { "slds-modal__footer": true }, key: 4 }, [api_slot("footer", { key: 5 }, [], $slotset)])])]`
}

func recordPickerTemplateJS() string {
	return `[api_element("label", { classMap: { "slds-form-element": true }, key: 0 }, [api_element("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [api_text($cmp.label || "")]), api_element("input", { classMap: { "slds-input": true }, attrs: { placeholder: $cmp.placeholder || "Search records" }, props: { value: $cmp.value || "" }, key: 2, on: { change: api_bind($cmp.handleRecordPickerChange), input: api_bind($cmp.handleRecordPickerChange) } })])]`
}

func treeTemplateJS() string {
	return `[api_element("ul", { classMap: { "slds-tree": true }, attrs: { role: "tree" }, key: 0 }, ($cmp.items || []).map((item, index) => api_element("li", { attrs: { role: "treeitem" }, key: 20 + index }, [api_text(item.label || item.name || "")])).concat([api_slot("", { key: 1 }, [], $slotset)]))]`
}

func treeGridTemplateJS() string {
	return `(() => { const columns = $cmp.columns || []; const rows = flattenTreeRows($cmp.data || []); return [api_element("table", { classMap: { "slds-table": true, "slds-tree": true }, attrs: { role: "treegrid" }, key: 0 }, [api_element("thead", { key: 1 }, [api_element("tr", { key: 2 }, columns.map((column, index) => api_element("th", { key: 20 + index }, [api_text(column.label || column.fieldName || "")])))]), api_element("tbody", { key: 3 }, rows.map((entry, rowIndex) => api_element("tr", { attrs: { "aria-level": String(entry.level + 1) }, key: 100 + rowIndex }, columns.map((column, colIndex) => api_element("td", { key: 1000 + rowIndex * 50 + colIndex }, [api_text((colIndex === 0 ? "  ".repeat(entry.level) : "") + ((entry.row && entry.row[column.fieldName]) ?? ""))])))))])]; })()`
}

func mapTemplateJS() string {
	return `[api_element("section", { classMap: { "slds-map": true }, attrs: { "data-zoom-level": String($cmp.zoomLevel || "") }, key: 0 }, [api_element("h3", { key: 1 }, [api_text($cmp.markersTitle || $cmp.title || "Map")]), api_element("ul", { key: 2 }, ($cmp.mapMarkers || $cmp.items || []).map((marker, index) => api_element("li", { key: 20 + index }, [api_text(markerText(marker))])))])]`
}

func carouselImageTemplateJS() string {
	return `[api_element("figure", { classMap: { "slds-carousel__panel": true }, key: 0 }, [api_element("img", { attrs: { src: $cmp.src || "", alt: $cmp.alternativeText || $cmp.header || "" }, key: 1 }), api_element("figcaption", { key: 2 }, [api_element("h3", { key: 3 }, [api_text($cmp.header || $cmp.label || "")]), api_element("p", { key: 4 }, [api_text($cmp.description || "")])])])]`
}

func verticalNavigationItemTemplateJS() string {
	return `[api_element("a", { classMap: { "slds-nav-vertical__action": true }, attrs: { href: $cmp.href || "#", role: "link" }, key: 0, on: { click: api_bind($cmp.handleActive) } }, [api_text($cmp.label || $cmp.name || "")])]`
}

func verticalNavigationItemBadgeTemplateJS() string {
	return `[api_element("a", { classMap: { "slds-nav-vertical__action": true }, attrs: { href: $cmp.href || "#", role: "link" }, key: 0, on: { click: api_bind($cmp.handleActive) } }, [api_text($cmp.label || $cmp.name || ""), api_element("span", { classMap: { "slds-badge": true }, attrs: { title: $cmp.assistiveText || "" }, key: 1 }, [api_text(String($cmp.badgeCount ?? ""))])])]`
}

func verticalNavigationItemIconTemplateJS() string {
	return `[api_element("a", { classMap: { "slds-nav-vertical__action": true }, attrs: { href: $cmp.href || "#", role: "link" }, key: 0, on: { click: api_bind($cmp.handleActive) } }, [api_element("span", { classMap: { "slds-icon_container": true }, key: 1 }, [api_text(($cmp.iconName || "").split(":").pop())]), api_text($cmp.label || $cmp.name || "")])]`
}

func verticalNavigationOverflowTemplateJS() string {
	return `[api_element("button", { classMap: { "slds-button": true, "slds-button_reset": true }, attrs: { type: "button" }, key: 0, on: { click: api_bind($cmp.handleActive) } }, [api_text($cmp.label || ($cmp.expanded ? "Show Less" : "Show More"))])]`
}
