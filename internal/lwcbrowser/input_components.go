package lwcbrowser

import "strings"

// Keep input constraint validation and events separate from the generic form
// methods used by record fields and other base components. Native input and
// textarea observations at APIs 59/67 are exported in l18_salesforce.json.
func lightningInputComponentModuleJS(def baseComponentDefinition) (string, bool) {
	textarea := "false"
	switch normalizeLightningBaseComponentName(def.Name) {
	case "input":
	case "textarea":
		textarea = "true"
	default:
		return "", false
	}
	return strings.NewReplacer(
		"__INPUT_CLASS__", def.ClassName,
		"__INPUT_TAG__", def.Tag,
		"__IS_TEXTAREA__", textarea,
	).Replace(lightningInputComponentJS), true
}

const lightningInputComponentJS = `import { LightningElement, registerDecorators, registerTemplate, freezeTemplate, registerComponent } from "lwc";
import locale from "@salesforce/i18n/locale";
import timeZone from "@salesforce/i18n/timeZone";
const isTextarea = __IS_TEXTAREA__;
const colorPattern = "^#([A-Fa-f0-9]{6}|[A-Fa-f0-9]{3})$";
const validityFlags = ["badInput", "customError", "patternMismatch", "rangeOverflow", "rangeUnderflow", "stepMismatch", "tooLong", "tooShort", "typeMismatch", "valueMissing"];
function createBaseComponent() {}
function booleanValue(value) {
  return typeof value === "string" || Boolean(value);
}
function dateText(value, zone = "UTC") {
  if (value == null || value === "") return "";
  const date = new Date(String(value));
  if (Number.isNaN(date.getTime())) return String(value);
  return new Intl.DateTimeFormat(locale, { timeZone: zone, year: "numeric", month: "short", day: "numeric" }).format(date);
}
function timeText(value) {
  if (value == null || value === "") return "";
  const input = document.createElement("input");
  input.type = "time";
  input.value = String(value);
  if (!input.value) return String(value);
  const date = new Date("1970-01-01T" + input.value + "Z");
  return new Intl.DateTimeFormat(locale, { timeZone: "UTC", hour: "numeric", minute: "2-digit" }).format(date).replace(/\u202f/g, " ");
}
function dateFormatExample() { return dateText("2024-12-31"); }
// Native API 68 captures (en_US and fr_FR): format messages and hints carry a
// concrete example, "11:30 AM" for a time, and read in the user's language.
// Only these two languages were captured; other locales keep the English text.
const isFrench = /^fr(?:[-_]|$)/i.test(String(locale));
// Release 68 captures (saved API 67.0 and 68.0) show the concrete example for a
// time too. The older "h:mm a" text, captured in a release 67 org, is kept as
// history in l18_salesforce.json (r_input_time_conversion). The text follows
// the org release, not the saved version.
function badInputMessage(isTime) {
  const example = isTime ? timeText("11:30") : dateFormatExample();
  return (isFrench ? "Votre entr\u00e9e ne correspond pas au format autoris\u00e9 " : "Your entry does not match the allowed format ") + example + ".";
}
// Dates are read the way dateText reads them: the native date input accepts
// values such as "42" (year 2042) that a detached date control would reject.
function unreadableDate(value) { return Number.isNaN(new Date(String(value)).getTime()); }
// Native API 68 captures (en_US and fr_FR): a date has a picker button whose
// assistive text names the field; a datetime shows Date and Time sub-labels
// ("Heure" in French) and a required datetime marks both with an asterisk.
function pickerText(label) { return (isFrench ? "S\u00e9lectionner une date pour " : "Select a date for ") + label; }
function timeLabelText() { return isFrench ? "Heure" : "Time"; }
function formatHint() { return "Format" + (isFrench ? "\u00a0: " : ": ") + dateFormatExample(); }
function tmpl($api, $cmp) {
  const { h: element, t: text, b: bind, f: flatten } = $api;
  const checkbox = $cmp.isCheckbox;
  const requiredMark = key => $cmp.required ? [element("abbr", { classMap: { "slds-required": true }, attrs: { "aria-hidden": "true", title: "required" }, key }, [text("*")])] : [];
  const children = [element("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [text($cmp.label || "")])];
  {
    const attrs = {
      name: $cmp.name,
      placeholder: $cmp.placeholder,
      "aria-label": $cmp.label,
      "aria-invalid": $cmp.type === "datetime-local" ? $cmp.dateAriaInvalid : $cmp.ariaInvalid,
      style: checkbox && $cmp.type === "checkbox" && $cmp.readOnly ? "display:none" : null,
      minlength: $cmp.minLength == null ? null : String($cmp.minLength),
      maxlength: $cmp.maxLength == null ? null : String($cmp.maxLength)
    };
    if (!isTextarea) {
      attrs.type = $cmp.controlType;
      attrs.pattern = $cmp.pattern;
      attrs.min = $cmp.min == null ? null : String($cmp.min);
      attrs.max = $cmp.max == null ? null : String($cmp.max);
      attrs.step = $cmp.step;
      if ($cmp.type === "time") attrs["aria-expanded"] = "false";
    }
    const props = {
      value: $cmp.type === "file" ? "" : $cmp.displayValue,
      disabled: $cmp.disabled,
      readOnly: $cmp.type === "color" ? false : $cmp.readOnly,
      required: $cmp.type === "color" ? false : $cmp.required
    };
    if (!isTextarea) props.checked = $cmp.checked;
    if ($cmp.type === "datetime-local") {
      children.push(element("span", { classMap: { "slds-form-element__label": true }, key: 14 }, flatten([requiredMark(15), text("Date")])));
    }
    children.push(element(isTextarea ? "textarea" : "input", {
      classMap: { "slds-input": !isTextarea, "slds-textarea": isTextarea },
      attrs, props, key: 2,
      on: { input: bind($cmp.handleInput), change: bind($cmp.handleChange), focus: bind($cmp.handleFocus), blur: bind($cmp.handleBlur), invalid: bind($cmp.handleInvalid) }
    }));
    if (["date", "datetime-local"].includes($cmp.type)) {
      const picker = pickerText($cmp.type === "datetime-local" ? "Date" : ($cmp.label || ""));
      children.push(element("button", { classMap: { "slds-button": true, "slds-button_icon": true }, attrs: { type: "button", title: picker }, key: 12 },
        [element("span", { classMap: { "slds-assistive-text": true }, key: 13 }, [text(picker)])]));
    }
  }
  const datetime = $cmp.type === "datetime-local";
  if ($cmp.type === "date" && !$cmp._message) {
    children.push(element("div", { classMap: { "slds-form-element__help": true }, key: 8 }, [text(formatHint())]));
  }
  if (datetime) {
    // The hint stays (assistive) unless the date half reports; then the date
    // half's message takes its place, ahead of the Time sub-label.
    if ($cmp._message && $cmp.dateHalfBad) {
      children.push(element("div", { classMap: { "slds-form-element__help": true }, key: 8 }, [element("span", { classMap: { "slds-assistive-text": true }, key: 4 }, [text("Date")]), text($cmp._message)]));
    } else if (!$cmp._message || $cmp.badHalf === "time") {
      // Captured: the hint stays for a time-half error; an accepted release 67
      // row (required and empty) shows no hint beside its missing-value message.
      children.push(element("div", { classMap: { "slds-form-element__help": true, "slds-assistive-text": true }, key: 8 }, [text(formatHint())]));
    }
    children.push(element("span", { classMap: { "slds-form-element__label": true }, key: 16 }, flatten([requiredMark(17), text(timeLabelText())])));
    children.push(element("input", {
      classMap: { "slds-input": true },
      attrs: { type: "text", "aria-label": $cmp.label, "aria-expanded": "false", "aria-invalid": $cmp.timeAriaInvalid },
      props: { value: $cmp.displayTime, checked: false, disabled: $cmp.disabled, readOnly: $cmp.readOnly, required: $cmp.required }, key: 7,
      on: { input: bind($cmp.handleTimeInput), focus: bind($cmp.handleFocus), blur: bind($cmp.handleBlur), invalid: bind($cmp.handleInvalid) }
    }));
    for (const key of [9, 10]) children.push(element("div", { attrs: { role: "alert" }, key }, []));
  }
  if (!isTextarea || $cmp._message) {
    const help = [];
    if ($cmp._message && !(datetime && $cmp.dateHalfBad)) {
      if (!$cmp.isTemporal || $cmp.type === "date") help.push(element("span", { classMap: { "slds-assistive-text": true }, key: 4 }, [text($cmp.label || "")]));
      help.push(text($cmp._message));
    }
    children.push(element("div", { classMap: { "slds-form-element__help": true }, attrs: { role: "alert" }, key: 3 }, help));
  }
  return [element("label", { classMap: { "slds-form-element": true, "slds-has-error": Boolean($cmp._message) }, key: 0 }, flatten(children))];
}
tmpl.stylesheets = [];
const template = registerTemplate(tmpl);
freezeTemplate(tmpl);

class __INPUT_CLASS__ extends LightningElement {
  constructor() {
    super();
    this._type = isTextarea ? undefined : "text";
    this._value = isTextarea ? undefined : "";
    this._checked = false;
    this._required = false;
    this._disabled = false;
    this._readOnly = false;
    this._step = undefined;
    this._pattern = undefined;
    this._customValidity = "";
    this._message = "";
    this._reported = false;
    this._files = null;
    this._committedValue = undefined;
    this._typedTime = undefined;
  }
  get type() { return this._type; }
  set type(value) {
    this._type = value === "datetime" ? "datetime-local" : (value == null ? "text" : String(value));
  }
  get value() { return this._value; }
  set value(value) {
    if (isTextarea) {
      this._value = value;
      return;
    }
    let normalized = String(value ?? "");
    if (this.type === "file") normalized = "";
    if (this.type === "number") {
      const input = document.createElement("input");
      input.type = "number";
      input.value = normalized;
      normalized = input.value;
    }
    this._value = normalized;
    this._typedTime = undefined;
  }
  get checked() { return this._checked; }
  set checked(value) { this._checked = booleanValue(value); }
  get required() { return this._required; }
  set required(value) { this._required = booleanValue(value); }
  get disabled() { return this._disabled; }
  set disabled(value) { this._disabled = booleanValue(value); }
  get readOnly() { return this._readOnly; }
  set readOnly(value) { this._readOnly = booleanValue(value); }
  get files() { return this._files; }
  get step() {
    if (this._step !== undefined) return this._step;
    return ["time", "datetime-local"].includes(this.type) ? "any" : undefined;
  }
  set step(value) { this._step = value == null ? undefined : String(value); }
  get pattern() { return this._pattern ?? (this.type === "color" ? colorPattern : undefined); }
  set pattern(value) { this._pattern = value; }
  get isCheckbox() { return ["checkbox", "checkbox-button", "toggle"].includes(this.type); }
  // Native API 68 captures: a datetime whose date half cannot be read reports
  // on the date; the time half stays silent. A bad time half reports its own.
  get dateHalfBad() {
    if (this.type !== "datetime-local" || !this.value) return false;
    return unreadableDate(String(this.value).split("T")[0]);
  }
  // Native API 68 capture: a time typed into the datetime time input that
  // cannot be read reports on the time half.
  get timeHalfBad() {
    return this.type === "datetime-local" && this._typedTime !== undefined && this._typedTime !== "" &&
      Number.isNaN(new Date("1970-01-01 " + this._typedTime).getTime());
  }
  // Which half a bad-input report belongs to: "date", "time", or null when
  // the report is not tied to one half (both inputs then show invalid).
  get badHalf() {
    if (!this._reported || this.validity.valid || !this.validity.badInput) return null;
    return this.dateHalfBad ? "date" : "time";
  }
  get dateAriaInvalid() {
    if (!this._reported || this.validity.valid) return null;
    return this.badHalf === "time" ? null : "true";
  }
  get timeAriaInvalid() {
    if (!this._reported || this.validity.valid) return null;
    return this.badHalf === "date" ? null : "true";
  }
  handleTimeInput(event) {
    event.stopPropagation();
    this._typedTime = event.target.value;
  }
  get isTemporal() { return ["date", "time", "datetime-local"].includes(this.type); }
  get controlType() {
    if (this.isCheckbox) return "checkbox";
    if (this.isTemporal || ["number", "email", "color"].includes(this.type)) return "text";
    return this.type || "text";
  }
  get displayValue() {
    if (this.type === "date") return dateText(this.value);
    if (this.type === "datetime-local") return dateText(this.value, timeZone);
    if (this.type === "time") return timeText(this.value);
    return String(this.value ?? "");
  }
  get displayTime() {
    if (this._typedTime !== undefined) return this._typedTime;
    if (!this.value || !String(this.value).includes("T")) return "";
    const date = new Date(this.value);
    if (Number.isNaN(date.getTime())) return "";
    return new Intl.DateTimeFormat(locale, { timeZone, hour: "numeric", minute: "2-digit" }).format(date).replace(/\u202f/g, " ");
  }
  constraintInput() {
    // A detached native control computes constraints without emitting events or
    // changing focus. Number/email display as text while their constraints keep
    // the public input type; color adds its observed text pattern.
    const input = document.createElement(isTextarea ? "textarea" : "input");
    if (!isTextarea) input.type = this.isCheckbox ? "checkbox" : (this.type === "color" ? "text" : this.type);
    for (const name of ["min", "max", "step", "minLength", "maxLength", "pattern"]) {
      const value = this[name];
      if (value != null) input.setAttribute(name.toLowerCase(), String(value));
    }
    input.required = this.required;
    input.disabled = this.disabled;
    input.readOnly = this.readOnly;
    input.multiple = Boolean(this.multiple);
    if (this.type !== "file") input.value = this.type === "datetime-local" ? String(this.value ?? "").replace(/Z$/, "") : String(this.value ?? "");
    input.checked = this.checked;
    return input;
  }
  get validity() {
    const input = this.constraintInput(), native = input.validity;
    const result = Object.fromEntries(validityFlags.map(flag => [flag, Boolean(native[flag])]));
    // Native email exposes a false typeMismatch flag even when valid is false.
    if (this.type === "email") result.typeMismatch = false;
    result.customError = this._customValidity !== "";
    result.valid = native.valid && !result.customError;
    // Native API 68 captures: an unreadable date or time value is bad input; an
    // unreadable datetime value is ignored unless the field is required.
    const unreadable = this.type === "time" ? !input.value : unreadableDate(this.value);
    if (this.timeHalfBad) {
      result.badInput = true;
      result.valid = false;
    }
    if (this.isTemporal && this.value && unreadable && (this.type !== "datetime-local" || this.required)) {
      result.badInput = true;
      result.valueMissing = false;
      result.valid = false;
    }
    return result;
  }
  get ariaInvalid() {
    if (this._reported && !this.validity.valid) return "true";
    if (!this.isCheckbox && !this.isTemporal && this.type !== "file" && this.value) return "false";
    return null;
  }
  setCustomValidity(message) { this._customValidity = message; }
  checkValidity() {
    const valid = this.validity.valid;
    if (!valid && !this.isTemporal) this.dispatchEvent(new CustomEvent("invalid"));
    return valid;
  }
  reportValidity() {
    const valid = this.checkValidity();
    this._reported = !valid;
    this._message = valid ? "" : this.constraintMessage();
    return valid;
  }
  constraintMessage() {
    const flags = this.validity;
    if (flags.customError) return this._customValidity == null ? "Enter a valid value." : String(this._customValidity);
    const defaults = {
      valueMissing: "Complete this field.",
      typeMismatch: this.type === "email" ? "Enter a valid email address, such as name@email.com." : "You have entered an invalid format.",
      patternMismatch: "Invalid Format",
      rangeUnderflow: "The number is too low.",
      rangeOverflow: "The number is too high.",
      stepMismatch: "Your entry isn't a valid increment."
    };
    if (this.isTemporal) {
      defaults.badInput = badInputMessage(this.type === "time" || (this.type === "datetime-local" && !this.dateHalfBad));
      if (this.type !== "time") defaults.valueMissing = "Complete this field with format " + dateFormatExample() + ".";
      for (const [flag, property, direction] of [["rangeUnderflow", "min", "later"], ["rangeOverflow", "max", "earlier"]]) {
        const bound = this.type === "date" ? dateText(this[property]) : this[property];
        defaults[flag] = "Value must be " + bound + " or " + direction + ".";
      }
    }
    for (const flag of ["badInput", "valueMissing", "typeMismatch", "patternMismatch", "tooShort", "tooLong", "rangeUnderflow", "rangeOverflow", "stepMismatch"]) {
      if (flags[flag] || (flag === "typeMismatch" && this.type === "email" && this.constraintInput().validity.typeMismatch)) {
        const property = "messageWhen" + flag[0].toUpperCase() + flag.slice(1);
        return this[property] || defaults[flag] || this.constraintInput().validationMessage;
      }
    }
    return "Enter a valid value.";
  }
  handleInvalid(event) { event.stopPropagation(); }
  handleFocus(event) {
    event.stopPropagation();
    this._committedValue = this.value;
    this.dispatchEvent(new CustomEvent("focus"));
  }
  handleBlur(event) {
    event.stopPropagation();
    this.dispatchEvent(new CustomEvent("blur"));
  }
  handleInput(event) {
    if (isTextarea) {
      this.handleChange(event);
      return;
    }
    event.stopPropagation();
    if (this.isCheckbox || this.type === "file") return;
    const value = event.target.value;
    if (value === this.value) return;
    this.value = value;
    this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value: this.value } }));
  }
  handleChange(event) {
    // Preserve the existing textarea change contract until an edit is captured.
    if (isTextarea) {
      if (event && event.stopPropagation) event.stopPropagation();
      const target = event && event.target || {};
      this.value = target.value;
      this.checked = Boolean(target.checked);
      this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value: target.value, checked: Boolean(target.checked) } }));
      return;
    }
    event.stopPropagation();
    if (this.isCheckbox) {
      this.checked = event.target.checked;
      this.dispatchEvent(new CustomEvent("commit"));
      this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { checked: this.checked } }));
    } else if (this.type === "file") {
      this.value = event.target.value;
      this.checked = Boolean(event.target.checked);
      this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value: event.target.value, checked: this.checked } }));
    } else if (this._committedValue !== this.value) {
      this._committedValue = this.value;
      this.dispatchEvent(new CustomEvent("commit"));
    }
  }
  focus() { this.template.querySelector("input,textarea")?.focus(); }
  blur() { this.template.querySelector("input,textarea")?.blur(); }
}
const publicProps = Object.fromEntries([
  "label", "name", "placeholder", "variant", "min", "max", "minLength", "maxLength", "multiple", "accept", "autocomplete",
  "messageWhenBadInput", "messageWhenValueMissing", "messageWhenTypeMismatch", "messageWhenPatternMismatch", "messageWhenTooShort", "messageWhenTooLong", "messageWhenRangeUnderflow", "messageWhenRangeOverflow", "messageWhenStepMismatch"
].map(name => [name, { config: 0 }]));
for (const name of ["value", "required", "disabled", "readOnly"]) publicProps[name] = { config: 3 };
publicProps.validity = { config: 1 };
if (!isTextarea) {
  for (const name of ["type", "checked", "step", "pattern"]) publicProps[name] = { config: 3 };
  publicProps.files = { config: 1 };
}
registerDecorators(__INPUT_CLASS__, {
  publicProps,
  publicMethods: ["setCustomValidity", "checkValidity", "reportValidity", "focus", "blur"],
  fields: ["_type", "_value", "_checked", "_required", "_disabled", "_readOnly", "_step", "_pattern", "_customValidity", "_message", "_reported", "_files", "_typedTime"]
});
export default registerComponent(__INPUT_CLASS__, { tmpl: template, sel: "__INPUT_TAG__" });
`
