package lwcbrowser

import "strings"

func lightningChoiceComponentModuleJS(def baseComponentDefinition) (string, bool) {
	kind := normalizeLightningBaseComponentName(def.Name)
	switch kind {
	case "combobox", "radiogroup", "select":
	default:
		return "", false
	}
	return strings.NewReplacer(
		"__CHOICE_CLASS__", def.ClassName,
		"__CHOICE_TAG__", def.Tag,
		"__CHOICE_KIND__", kind,
	).Replace(lightningChoiceComponentJS), true
}

// Native choice components retain assigned values and accept custom errors only
// as strings. Unlike input/textarea, null and numeric custom messages clear the
// error. Their rendered controls and public change event contracts also differ.
const lightningChoiceComponentJS = `import { LightningElement, registerDecorators, registerTemplate, freezeTemplate, registerComponent } from "lwc";
const kind = "__CHOICE_KIND__";
function createBaseComponent() {}
function optionValue(option) { return String(option.value ?? option.label ?? ""); }
function tmpl($api, $cmp) {
  const { h: element, t: text, b: bind, f: flatten } = $api;
  const label = element(kind === "radiogroup" ? "legend" : "span", { classMap: { "slds-form-element__label": true }, key: 1 }, [text($cmp.label || "")]);
  const controls = [];
  if (kind === "radiogroup") {
    for (const [index, option] of ($cmp.options || []).entries()) {
      const value = optionValue(option);
      controls.push(element("label", { classMap: { "slds-radio": true }, key: 20 + index }, [
        element("input", { attrs: { type: "radio", name: $cmp.name || $cmp.label || "radio", value }, props: { checked: $cmp.value === value, disabled: $cmp.disabled }, key: 200 + index, on: { change: bind($cmp.handleChange), focus: bind($cmp.handleFocus), blur: bind($cmp.handleBlur) } }),
        element("span", { key: 400 + index }, [text(option.label ?? option.value ?? "")])
      ]));
    }
  } else if (kind === "select") {
    controls.push(element("select", {
      classMap: { "slds-select": true }, attrs: { name: $cmp.name, "aria-label": $cmp.label, "aria-invalid": $cmp._message ? "true" : null },
      props: { value: String($cmp.value ?? ""), required: $cmp.required, disabled: $cmp.disabled }, key: 2,
      on: { change: bind($cmp.handleChange), focus: bind($cmp.handleFocus), blur: bind($cmp.handleBlur), invalid: bind($cmp.handleInvalid) }
    }, ($cmp.options || []).map((option, index) => element("option", { attrs: { value: optionValue(option) }, props: { selected: $cmp.value === optionValue(option) }, key: 20 + index }, [text(option.label ?? option.value ?? "")]))));
  } else {
    const selected = ($cmp.options || []).find(option => $cmp.value === optionValue(option));
    controls.push(element("button", {
      classMap: { "slds-combobox__input": true, "slds-button": true, "slds-button_neutral": true },
      attrs: { type: "button", role: "combobox", "aria-label": $cmp.label, "aria-expanded": String($cmp._open), "aria-haspopup": "listbox", "aria-invalid": $cmp._message ? "true" : null },
      props: { disabled: $cmp.disabled }, key: 2,
      on: { click: bind($cmp.handleOpen), focus: bind($cmp.handleFocus), blur: bind($cmp.handleBlur) }
    }, [text(selected?.label ?? $cmp.placeholder ?? "Select an Option")]));
    if ($cmp._open) {
      controls.push(element("div", { classMap: { "slds-dropdown": true, "slds-dropdown_fluid": true }, attrs: { role: "listbox", style: "position:static;display:block" }, key: 3 },
        ($cmp.options || []).map((option, index) => element("div", {
          classMap: { "slds-listbox__option": true },
          attrs: { role: "option", "data-value": optionValue(option), "aria-selected": String($cmp.value === optionValue(option)) }, key: 20 + index,
          on: { mousedown: bind($cmp.handleOptionMouseDown), click: bind($cmp.handleChoose) }
        }, [text(option.label ?? option.value ?? "")]))));
    }
  }
  const help = $cmp._message ? [
    element("span", { classMap: { "slds-assistive-text": true }, key: 6 }, [text($cmp.label || "")]), text($cmp._message)
  ] : [];
  return [element(kind === "radiogroup" ? "fieldset" : "div", { classMap: { "slds-form-element": true, "slds-has-error": Boolean($cmp._message) }, key: 0 }, [
    label,
    element("div", { classMap: { "slds-form-element__control": true, "slds-combobox": kind === "combobox", "slds-is-open": $cmp._open }, key: 4 }, flatten(controls)),
    element("div", { classMap: { "slds-form-element__help": true }, attrs: { role: "alert" }, key: 5 }, help)
  ])];
}
tmpl.stylesheets = [];
const template = registerTemplate(tmpl);
freezeTemplate(tmpl);
class __CHOICE_CLASS__ extends LightningElement {
  constructor() {
    super();
    this._value = undefined;
    this._required = false;
    this._disabled = false;
    this._readOnly = false;
    this._type = "radio";
    this._customValidity = "";
    this._message = "";
    this._open = false;
  }
  get value() { return this._value; }
  set value(value) { this._value = value; }
  get required() { return this._required; }
  set required(value) { this._required = typeof value === "string" || Boolean(value); }
  get disabled() { return this._disabled; }
  set disabled(value) { this._disabled = typeof value === "string" || Boolean(value); }
  get readOnly() { return this._disabled || this._readOnly; }
  set readOnly(value) { this._readOnly = typeof value === "string" || Boolean(value); }
  get type() { return this._type; }
  set type(value) { this._type = value; }
  get validity() {
    const result = Object.fromEntries(["badInput", "customError", "patternMismatch", "rangeOverflow", "rangeUnderflow", "stepMismatch", "tooLong", "tooShort", "typeMismatch", "valueMissing"].map(flag => [flag, false]));
    result.customError = this._customValidity !== "";
    result.valueMissing = !this.disabled && this.required && (this.value == null || this.value === "");
    result.valid = !result.customError && !result.valueMissing;
    return result;
  }
  setCustomValidity(message) { this._customValidity = typeof message === "string" ? message : ""; }
  checkValidity() {
    const valid = this.validity.valid;
    if (!valid) this.dispatchEvent(new CustomEvent("invalid"));
    return valid;
  }
  reportValidity() {
    const valid = this.checkValidity();
    if (kind === "combobox") this.checkValidity();
    this._message = valid ? "" : (this.validity.customError ? this._customValidity : (this.messageWhenValueMissing || "Complete this field."));
    return valid;
  }
  handleInvalid(event) { event.stopPropagation(); }
  handleFocus(event) {
    event.stopPropagation();
    this.dispatchEvent(new CustomEvent("focus"));
  }
  handleBlur(event) {
    event.stopPropagation();
    this._open = false;
    this.dispatchEvent(new CustomEvent("blur"));
  }
  handleChange(event) {
    event.stopPropagation();
    this.value = event.target.value;
    this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, cancelable: kind === "radiogroup", detail: { value: this.value } }));
  }
  handleOpen(event) {
    event.stopPropagation();
    if (this.disabled || this.readOnly) return;
    this._open = !this._open;
    if (this._open) this.dispatchEvent(new CustomEvent("open"));
  }
  handleOptionMouseDown(event) { event.preventDefault(); }
  handleChoose(event) {
    event.stopPropagation();
    this.dispatchEvent(new CustomEvent("focus"));
    this.value = event.currentTarget.dataset.value;
    this._open = false;
    this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value: this.value } }));
  }
  focus() { this.template.querySelector("input,select,button")?.focus(); }
  blur() { this.template.querySelector("input,select,button")?.blur(); }
}
const publicProps = Object.fromEntries(["label", "name", "options", "variant", "placeholder", "messageWhenValueMissing"].map(name => [name, { config: 0 }]));
for (const name of ["value", "required", "disabled"]) publicProps[name] = { config: 3 };
publicProps.validity = { config: 1 };
if (kind === "combobox") publicProps.readOnly = { config: 3 };
if (kind === "radiogroup") publicProps.type = { config: 3 };
registerDecorators(__CHOICE_CLASS__, {
  publicProps, publicMethods: ["setCustomValidity", "checkValidity", "reportValidity", "focus", "blur"],
  fields: ["_value", "_required", "_disabled", "_readOnly", "_type", "_customValidity", "_message", "_open"]
});
export default registerComponent(__CHOICE_CLASS__, { tmpl: template, sel: "__CHOICE_TAG__" });
`
