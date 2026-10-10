import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import { __gladeRecordPickerSearch, getRecord } from "lightning/uiRecordApi";

const DEFAULT_FIELDS = ["Name"];
const CONFIGURATION_ERROR = "This field can't load because of a configuration problem. Ask your Salesforce admin for help.";
const FILTER_OPERATORS = new Set(["eq", "ne", "lt", "gt", "lte", "gte", "like", "in", "nin", "includes", "excludes"]);

function validConfiguration(component) {
  for (const info of [component.displayInfo, component.matchingInfo]) {
    if (info != null && !fieldPath(info.primaryField)) return false;
  }
  const filter = component.filter;
  return filter == null || (Array.isArray(filter.criteria) && filter.criteria.length > 0 &&
    filter.criteria.every(criterion => fieldPath(criterion) && FILTER_OPERATORS.has(criterion.operator)));
}

function fieldPath(value) {
  if (!value) {
    return "";
  }
  if (typeof value === "string") {
    return value;
  }
  return value.fieldPath || value.fieldApiName || value.name || "";
}

function addField(fields, seen, value) {
  const name = fieldPath(value).trim();
  if (!name) {
    return;
  }
  const key = name.toLowerCase();
  if (seen.has(key)) {
    return;
  }
  seen.add(key);
  fields.push(name);
}

function normalizeFields(...sources) {
  const fields = [];
  const seen = new Set();
  for (const source of sources) {
    if (Array.isArray(source)) {
      for (const entry of source) {
        addField(fields, seen, entry);
      }
      continue;
    }
    addField(fields, seen, source);
  }
  return fields;
}

function matchingFields(matchingInfo) {
  if (!matchingInfo) {
    return DEFAULT_FIELDS;
  }
  const fields = normalizeFields(
    matchingInfo.primaryField,
    matchingInfo.additionalFields,
  );
  return fields.length ? fields : DEFAULT_FIELDS;
}

function displayFields(displayInfo, matchFields) {
  if (!displayInfo) {
    return matchFields.length ? matchFields : DEFAULT_FIELDS;
  }
  const fields = normalizeFields(
    displayInfo.primaryField,
    displayInfo.additionalFields,
    matchFields,
  );
  return fields.length ? fields : DEFAULT_FIELDS;
}

function fieldDisplay(record, fieldName) {
  const fields = record?.fields || {};
  const field = fields[fieldName] || fields[fieldName.split(".").pop()];
  if (!field || typeof field !== "object") {
    return field == null ? "" : String(field);
  }
  const value = field.displayValue ?? field.value;
  return value == null ? "" : String(value);
}

function recordTitle(record) {
  return record?.title || fieldDisplay(record, "Name") || record?.id || "";
}

function recordSubtitle(record, fields) {
  const title = recordTitle(record);
  return fields
    .map((field) => fieldDisplay(record, field))
    .filter((value) => value && value !== title)
    .join(" - ");
}

function renderRecordPicker($api, $cmp) {
  const { h, t, b } = $api;
  const rows = ($cmp.recordPickerRecords || []).map((record, index) => {
    const title = recordTitle(record);
    const subtitle = recordSubtitle(record, $cmp.displayFieldNames || DEFAULT_FIELDS);
    const children = [
      h("span", { classMap: { "slds-media__body": true }, key: 30 + index }, [
        h("span", { classMap: { "slds-listbox__option-text": true, "slds-listbox__option-text_entity": true }, key: 40 + index }, [t(title)]),
        subtitle ? h("span", { classMap: { "slds-listbox__option-meta": true, "slds-listbox__option-meta_entity": true }, key: 50 + index }, [t(subtitle)]) : null,
      ]),
    ].filter(Boolean);
    return h("li", { attrs: { role: "presentation" }, key: 100 + index }, [
      h("button", {
        classMap: { "slds-listbox__option": true, "slds-listbox__option_entity": true, "slds-button": true, "slds-button_reset": true },
        attrs: {
          type: "button",
          role: "option",
          "data-record-id": record.id || "",
        },
        key: 200 + index,
        on: { click: b($cmp.handleResultClick), mousedown: b($cmp.handleResultClick) },
      }, children),
    ]);
  });
  const hasRows = rows.length > 0;
  const clearLabel = "Clear " + ($cmp.label || "") + " Selection";
  const status = $cmp.validationMessage ? ($cmp.label || "") + "\n" + $cmp.validationMessage : "";
  return [h("div", { classMap: { "slds-form-element": true }, key: 0 }, [
    h("label", { classMap: { "slds-form-element__label": true }, attrs: { for: "record-picker-input" }, key: 1 }, [t($cmp.label || "")]),
    h("div", { classMap: { "slds-form-element__control": true }, key: 2 }, $api.f([
      h("input", {
        classMap: { "slds-input": true },
        attrs: {
          id: "record-picker-input",
          type: "text",
          placeholder: $cmp.loadingRecord ? "Loading..." : ($cmp.placeholder || ""),
          "aria-invalid": status ? "true" : null,
          role: $cmp.objectApiName ? "combobox" : null,
          "aria-expanded": $cmp.objectApiName ? String(hasRows) : null,
          "aria-autocomplete": $cmp.objectApiName ? "list" : null,
        },
        props: { value: $cmp.inputValue, disabled: Boolean($cmp.disabled || $cmp.configurationError || $cmp.loadingRecord),
          required: Boolean($cmp.required), readOnly: Boolean($cmp.selectedRecord) },
        key: 3,
        on: { change: b($cmp.handleTextChange), input: b($cmp.handleInput), focus: b($cmp.handleFocus), blur: b($cmp.handleBlur) },
      }),
      $cmp.selectedRecord ? h("button", { attrs: { type: "button", title: clearLabel },
        props: { disabled: Boolean($cmp.disabled) }, key: 7 }, [t(clearLabel)]) : null,
      h("div", { classMap: { "slds-form-element__help": true }, attrs: { style: "white-space: pre-line" }, key: 4 }, [t(status)]),
      $cmp.configurationError ? h("div", { attrs: { role: "alert" }, key: 8 }, [t(CONFIGURATION_ERROR)]) : null,
      hasRows ? h("div", { classMap: { "slds-dropdown": true, "slds-dropdown_fluid": true, "slds-dropdown_length-5": true }, key: 5 }, [
        h("ul", { classMap: { "slds-listbox": true, "slds-listbox_vertical": true }, attrs: { role: "listbox" }, key: 6 }, rows),
      ]) : null,
    ].filter(Boolean))),
  ])];
}

class GladeRecordPicker extends LightningElement {
  constructor(...args) {
    super(...args);
    this.label = "";
    this.objectApiName = "";
    this.placeholder = "";
    this.value = undefined;
    this.disabled = false;
    this.required = false;
    this.variant = "standard";
    this.filter = null;
    this.matchingInfo = null;
    this.displayInfo = null;
    this.recordPickerRecords = [];
    this.displayFieldNames = DEFAULT_FIELDS;
    this.searchTerm = "";
    this.searching = false;
    this.errorMessage = "";
    this.loadingRecord = false;
    this.selectedRecord = undefined;
    this.validationMessage = "";
    this.__customValidity = "";
    this.__searchToken = 0;
  }

  renderedCallback() {
    const configuration = JSON.stringify([this.objectApiName, this.label, this.value, this.displayInfo, this.matchingInfo, this.filter]);
    if (configuration === this.__configuration) return;
    this.__configuration = configuration;
    this.__recordAdapter?.disconnect();
    this.__recordAdapter = undefined;
    this.selectedRecord = undefined;
    this.loadingRecord = Boolean(this.value);
    if (this.configurationError) {
      this.loadingRecord = false;
      // Missing labels fail both the required public input and the picker
      // configuration, as captured separately from invalid search settings.
      if (!this.label) this.dispatchEvent(new CustomEvent("error"));
      this.dispatchEvent(new CustomEvent("error"));
      return;
    }
    if (!this.value) {
      this.dispatchReady();
      return;
    }
    // Non-string selections retain their public value and a pending control;
    // they do not become an ID or an invented record-read failure.
    if (typeof this.value !== "string") return;
    const recordId = this.value;
    this.__recordAdapter = new getRecord(({ data }) => {
      if (this.__configuration !== configuration) return;
      if (data) {
        this.selectedRecord = data;
        this.loadingRecord = false;
        this.dispatchReady();
      }
    });
    this.__recordAdapter.connect();
    this.__recordAdapter.update({ recordId, fields: displayFields(this.displayInfo, DEFAULT_FIELDS).map(name => `${this.objectApiName}.${name}`) });
  }

  disconnectedCallback() {
    this.__recordAdapter?.disconnect();
    this.__configuration = undefined;
    this.__searchToken += 1;
  }

  get inputValue() {
    return this.value && this.selectedRecord ? fieldDisplay(this.selectedRecord, fieldPath(this.displayInfo?.primaryField) || "Name") : this.searchTerm;
  }

  get configurationError() {
    return !this.label || !validConfiguration(this);
  }

  dispatchReady() {
    this.dispatchEvent(new CustomEvent("ready"));
  }

  handleFocus(event) {
    event.stopPropagation();
    this.dispatchEvent(new CustomEvent("focus"));
  }

  handleBlur(event) {
    event.stopPropagation();
    this.dispatchEvent(new CustomEvent("blur"));
  }

  focus() {
    this.template.querySelector("input")?.focus();
  }

  setCustomValidity(message) {
    this.__customValidity = String(message || "");
  }

  reportValidity() {
    this.validationMessage = this.__customValidity || (this.required && !this.value ? "Complete this field." : "");
    return !this.validationMessage;
  }

  handleTextChange(event) {
    event.stopPropagation();
    // Committing search text is not a record selection. The input event owns
    // the request; its results remain available for selecting a suggestion.
  }

  handleInput(event) {
    event?.stopPropagation?.();
    const term = event?.target?.value || "";
    this.searchTerm = term;
    if (!this.objectApiName) {
      this.value = term;
      this.recordPickerRecords = [];
      this.dispatchChange(term);
      return;
    }
    this.search(term);
  }

  handleResultClick(event) {
    event?.stopPropagation?.();
    event?.preventDefault?.();
    const recordId = event?.currentTarget?.dataset?.recordId || "";
    if (!recordId || (recordId === this.value && this.recordPickerRecords.length === 0)) {
      return;
    }
    const record = this.recordPickerRecords.find((entry) => entry.id === recordId);
    this.value = recordId;
    this.selectedRecord = record;
    this.searchTerm = "";
    this.recordPickerRecords = [];
    this.dispatchChange(recordId);
  }

  dispatchChange(recordId) {
    this.dispatchEvent(new CustomEvent("change", {
      bubbles: true,
      composed: true,
      detail: { recordId, value: recordId },
    }));
  }

  async search(term) {
    const token = ++this.__searchToken;
    const matchFields = matchingFields(this.matchingInfo);
    const fields = displayFields(this.displayInfo, matchFields);
    this.displayFieldNames = fields;
    this.searching = true;
    this.errorMessage = "";
    try {
      const result = await __gladeRecordPickerSearch({
        objectApiName: this.objectApiName,
        searchTerm: term,
        fields,
        matchingFields: matchFields,
        pageSize: 10,
      });
      if (token !== this.__searchToken) {
        return;
      }
      this.recordPickerRecords = Array.isArray(result?.records) ? result.records : [];
    } catch (err) {
      if (token !== this.__searchToken) {
        return;
      }
      this.recordPickerRecords = [];
      this.errorMessage = "Something went wrong. Try again.";
      this.validationMessage = this.errorMessage;
      this.dispatchEvent(new CustomEvent("error"));
    } finally {
      if (token === this.__searchToken) {
        this.searching = false;
      }
    }
  }
}

registerDecorators(GladeRecordPicker, {
  publicProps: {
    label: { config: 0 },
    objectApiName: { config: 0 },
    placeholder: { config: 0 },
    value: { config: 0 },
    disabled: { config: 0 },
    required: { config: 0 },
    variant: { config: 0 },
    filter: { config: 0 },
    matchingInfo: { config: 0 },
    displayInfo: { config: 0 },
  },
  publicMethods: ["focus", "setCustomValidity", "reportValidity"],
  track: {
    recordPickerRecords: 1,
    displayFieldNames: 1,
    searchTerm: 1,
    searching: 1,
    errorMessage: 1,
    loadingRecord: 1,
    selectedRecord: 1,
    validationMessage: 1,
  },
});

renderRecordPicker.stylesheets = [];
const template = registerTemplate(renderRecordPicker);
freezeTemplate(renderRecordPicker);

export default registerComponent(GladeRecordPicker, { tmpl: template, sel: "lightning-record-picker", apiVersion: 63 });
