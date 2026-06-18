import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import { __gladeRecordPickerSearch } from "lightning/uiRecordApi";

const DEFAULT_FIELDS = ["Name"];

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
  const status = $cmp.searching ? "Searching..." : ($cmp.errorMessage || "");
  return [h("div", { classMap: { "slds-form-element": true }, key: 0 }, [
    h("label", { classMap: { "slds-form-element__label": true }, attrs: { for: "record-picker-input" }, key: 1 }, [t($cmp.label || "")]),
    h("div", { classMap: { "slds-form-element__control": true }, key: 2 }, [
      h("input", {
        classMap: { "slds-input": true },
        attrs: {
          id: "record-picker-input",
          placeholder: $cmp.placeholder || "Search records",
          disabled: $cmp.disabled ? "" : null,
          role: $cmp.objectApiName ? "combobox" : null,
          "aria-expanded": $cmp.objectApiName ? String(hasRows) : null,
          "aria-autocomplete": $cmp.objectApiName ? "list" : null,
        },
        props: { value: $cmp.inputValue },
        key: 3,
        on: { change: b($cmp.handleInput), input: b($cmp.handleInput) },
      }),
      status ? h("div", { classMap: { "slds-form-element__help": true }, key: 4 }, [t(status)]) : null,
      hasRows ? h("div", { classMap: { "slds-dropdown": true, "slds-dropdown_fluid": true, "slds-dropdown_length-5": true }, key: 5 }, [
        h("ul", { classMap: { "slds-listbox": true, "slds-listbox_vertical": true }, attrs: { role: "listbox" }, key: 6 }, rows),
      ]) : null,
    ]),
  ])];
}

class GladeRecordPicker extends LightningElement {
  constructor(...args) {
    super(...args);
    this.label = "";
    this.objectApiName = "";
    this.placeholder = "";
    this.value = "";
    this.disabled = false;
    this.matchingInfo = null;
    this.displayInfo = null;
    this.recordPickerRecords = [];
    this.displayFieldNames = DEFAULT_FIELDS;
    this.searchTerm = "";
    this.searching = false;
    this.errorMessage = "";
    this.__searchToken = 0;
  }

  get inputValue() {
    return this.searchTerm || this.value || "";
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
    this.searchTerm = recordTitle(record);
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
      this.errorMessage = err?.message || "Record picker search failed";
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
    matchingInfo: { config: 0 },
    displayInfo: { config: 0 },
  },
  track: {
    recordPickerRecords: 1,
    displayFieldNames: 1,
    searchTerm: 1,
    searching: 1,
    errorMessage: 1,
  },
});

renderRecordPicker.stylesheets = [];
const template = registerTemplate(renderRecordPicker);
freezeTemplate(renderRecordPicker);

export default registerComponent(GladeRecordPicker, { tmpl: template, sel: "lightning-record-picker", apiVersion: 63 });
