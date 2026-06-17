import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import { reportDiagnostic } from "@glade/shell/diagnostics";

const PUBLIC_PROPS = [
  "label",
  "title",
  "value",
  "options",
  "checked",
  "disabled",
  "type",
  "variant",
  "iconName",
  "alternativeText",
  "size",
  "columns",
  "data",
  "keyField",
  "objectApiName",
  "recordId",
  "fields",
  "mode",
  "name",
  "fieldName",
  "error",
];

const UNSUPPORTED_ATTRIBUTE_NAMES = new Set([
  "hide-checkbox-column",
  "max-row-selection",
  "sorted-by",
  "sorted-direction",
  "show-row-number-column",
  "wrap-text-max-lines",
]);

function publicProps() {
  const props = {};
  for (const name of PUBLIC_PROPS) {
    props[name] = { config: 0 };
  }
  return props;
}

export function createBaseComponent(selector, render, options = {}) {
  class GladeBaseComponent extends LightningElement {
    reportUnsupportedAttributes() {
      const unsupportedAttrs = unsupportedBaseAttributes(this);
      if (!unsupportedAttrs.length) {
        return;
      }
      const message = `GLADELWC061 base component attributes unsupported locally: ${unsupportedAttrs.join(", ")}`;
      reportDiagnostic({ code: "GLADELWC061", severity: "warning", message, tagName: selector, attributes: unsupportedAttrs });
    }

    handleChange(event) {
      const target = event?.target || {};
      this.value = target.value;
      this.checked = target.checked;
      this.dispatchEvent(new CustomEvent("change", {
        bubbles: true,
        composed: true,
        detail: { value: target.value, checked: target.checked },
      }));
    }

    handleSubmit(event) {
      event.preventDefault();
      const fields = this.collectRecordFormFields();
      const submitEvent = new CustomEvent("submit", {
        bubbles: true,
        composed: true,
        cancelable: true,
        detail: { fields },
      });
      this.dispatchEvent(submitEvent);
      if (!isRecordFormSelector(selector) || submitEvent.defaultPrevented) {
        return;
      }
      fetch("/lightning/wire/updateRecord", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({ fields: { Id: this.recordId, ...fields } }),
      }).then((response) => response.json()).then((result) => {
        if (result?.error) {
          this.error = result.error;
          this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail: result.error }));
          return;
        }
        if (result?.data) {
          this.value = result.data;
        }
        this.dispatchEvent(new CustomEvent("success", {
          bubbles: true,
          composed: true,
          detail: { id: result?.data?.id || this.recordId, fields },
        }));
      }).catch((err) => {
        const detail = { message: err?.message || String(err) };
        this.error = detail;
        this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail }));
      });
    }

    collectRecordFormFields() {
      const fields = {};
      const inputs = this.template?.querySelectorAll?.("[data-field-name]") || [];
      for (const input of inputs) {
        const name = input.dataset?.fieldName;
        if (!name) {
          continue;
        }
        fields[name] = input.type === "checkbox" ? Boolean(input.checked) : input.value;
      }
      return fields;
    }

    handleRowAction(event) {
      const dataset = event?.currentTarget?.dataset || {};
      const rowIndex = Number(dataset.rowIndex);
      const columnIndex = Number(dataset.columnIndex);
      const actionIndex = Number(dataset.actionIndex);
      const rows = this.data || [];
      const column = (this.columns || [])[columnIndex];
      const actions = column?.typeAttributes?.rowActions || [];
      const row = rows[rowIndex];
      const action = actions[actionIndex];
      if (!row || !action) {
        return;
      }
      this.dispatchEvent(new CustomEvent("rowaction", { bubbles: true, composed: true, detail: { action, row } }));
    }

    handleActive(event) {
      event?.preventDefault?.();
      this.dispatchEvent(new CustomEvent("active", {
        bubbles: true,
        composed: true,
        detail: { value: this.value || this.name || this.label || "", label: this.label || "" },
      }));
    }

    connectedCallback() {
      this.reportUnsupportedAttributes();
      if (options.unsupported) {
        const message = `GLADELWC060 base component unsupported: ${selector}`;
        reportDiagnostic({ code: "GLADELWC060", severity: "warning", message, tagName: selector });
        throw new Error(message);
      }
      if (isRecordFormSelector(selector)) {
        this.loadRecordFormRecord();
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
        body: JSON.stringify({
          recordId: this.recordId,
          fields: recordFormFieldRefs(this.objectApiName, this.fields),
        }),
      }).then((response) => response.json()).then((result) => {
        if (result?.error) {
          this.error = result.error;
          this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail: result.error }));
          return;
        }
        this.value = result?.data || result;
      }).catch((err) => {
        const detail = { message: err?.message || String(err) };
        this.error = detail;
        this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail }));
      });
    }
  }

  registerDecorators(GladeBaseComponent, { publicProps: publicProps() });
  render.stylesheets = [];
  render.slots = [""];
  const template = registerTemplate(render);
  freezeTemplate(render);
  return registerComponent(GladeBaseComponent, { tmpl: template, sel: selector, apiVersion: 63 });
}

export function unsupportedBaseComponent(selector) {
  const message = `GLADELWC060 base component unsupported: ${selector}`;
  reportDiagnostic({ code: "GLADELWC060", severity: "warning", message, tagName: selector });
  throw new Error(message);
}

export function unsupportedBaseAttributes(component) {
  const host = component?.hostElement || component;
  if (!host || typeof host.getAttributeNames !== "function") {
    return [];
  }
  const unsupportedAttrs = [];
  for (const name of host.getAttributeNames()) {
    if (UNSUPPORTED_ATTRIBUTE_NAMES.has(name)) {
      unsupportedAttrs.push(name);
    }
  }
  return unsupportedAttrs;
}

export function text(api, value) {
  return api.t(String(value ?? ""));
}

export function iconLabel(name) {
  const value = String(name || "utility:placeholder");
  const parts = value.split(":");
  return parts[parts.length - 1] || value;
}

export function fieldList(fields) {
  if (Array.isArray(fields)) {
    return fields;
  }
  if (typeof fields === "string" && fields.trim()) {
    return fields.split(",").map((field) => field.trim());
  }
  return [];
}

function isRecordFormSelector(selector) {
  return selector === "lightning-record-form" ||
    selector === "lightning-record-view-form" ||
    selector === "lightning-record-edit-form";
}

function fieldApiName(field) {
  const value = typeof field === "string" ? field : (field?.fieldApiName || field?.apiName || field?.fieldName || field?.name || "");
  const parts = String(value || "").split(".");
  return parts[parts.length - 1] || "";
}

function recordFormFieldRefs(objectApiName, fields) {
  return fieldList(fields).map((field) => {
    const raw = typeof field === "string" ? field : (field?.fieldApiName || field?.apiName || field?.fieldName || field?.name || "");
    if (!raw) {
      return "";
    }
    if (String(raw).includes(".")) {
      return String(raw);
    }
    return `${objectApiName}.${raw}`;
  }).filter(Boolean);
}

function recordFieldDisplayValue(record, field) {
  const name = fieldApiName(field);
  const value = record?.fields?.[name];
  if (value && typeof value === "object") {
    return value.displayValue ?? value.value ?? "";
  }
  return value ?? "";
}

export function renderButton($api, $cmp) {
  const { h, t } = $api;
  return [h("button", {
    classMap: { "slds-button": true, "slds-button_neutral": true },
    attrs: { type: $cmp.type || "button" },
    props: { disabled: Boolean($cmp.disabled) },
    key: 0,
  }, [t($cmp.label || "")])];
}

export function renderButtonIcon($api, $cmp) {
  const { h, t } = $api;
  return [h("button", {
    classMap: { "slds-button": true, "slds-button_icon": true },
    attrs: { type: "button", title: $cmp.alternativeText || $cmp.iconName || "" },
    props: { disabled: Boolean($cmp.disabled) },
    key: 0,
  }, [t(iconLabel($cmp.iconName))])];
}

export function renderCard($api, $cmp, $slotset) {
  const { h, t, s } = $api;
  return [h("article", { classMap: { "slds-card": true }, key: 0 }, [
    h("header", { classMap: { "slds-card__header": true }, key: 1 }, [
      h("h2", { classMap: { "slds-card__header-title": true }, key: 2 }, [t($cmp.title || "")]),
    ]),
    h("div", { classMap: { "slds-card__body": true }, key: 3 }, [s("", { key: 4 }, [], $slotset)]),
  ])];
}

export function renderInput($api, $cmp) {
  const { h, t, b } = $api;
  return [h("label", { classMap: { "slds-form-element": true }, key: 0 }, [
    h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t($cmp.label || "")]),
    h("input", {
      classMap: { "slds-input": true },
      attrs: { type: $cmp.type || "text" },
      props: { value: $cmp.value || "", disabled: Boolean($cmp.disabled) },
      key: 2,
      on: { change: b($cmp.handleChange), input: b($cmp.handleChange) },
    }),
  ])];
}

export function renderTextarea($api, $cmp) {
  const { h, t, b } = $api;
  return [h("label", { classMap: { "slds-form-element": true }, key: 0 }, [
    h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t($cmp.label || "")]),
    h("textarea", {
      classMap: { "slds-textarea": true },
      props: { value: $cmp.value || "" },
      key: 2,
      on: { change: b($cmp.handleChange), input: b($cmp.handleChange) },
    }),
  ])];
}

export function renderCombobox($api, $cmp) {
  const { h, t, b } = $api;
  const value = String($cmp.value ?? "");
  return [h("label", { classMap: { "slds-form-element": true, "slds-combobox": true }, key: 0 }, [
    h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t($cmp.label || "")]),
    h("select", {
      classMap: { "slds-select": true },
      props: { value },
      key: 2,
      on: { change: b($cmp.handleChange) },
    }, ($cmp.options || []).map((option, index) => {
      const optionValue = String(option.value ?? option.label ?? "");
      return h("option", {
        attrs: { value: optionValue },
        props: { selected: optionValue === value },
        key: 20 + index,
      }, [t(option.label || option.value || "")]);
    })),
  ])];
}

export function renderLayout($api, _cmp, $slotset) {
  return [$api.h("div", { classMap: { "slds-grid": true, "slds-wrap": true }, key: 0 }, [
    $api.s("", { key: 1 }, [], $slotset),
  ])];
}

export function renderLayoutItem($api, _cmp, $slotset) {
  return [$api.h("div", { classMap: { "slds-col": true }, key: 0 }, [
    $api.s("", { key: 1 }, [], $slotset),
  ])];
}

export function renderTabset($api, _cmp, $slotset) {
  return [$api.h("div", { classMap: { "slds-tabs_default": true }, key: 0 }, [
    $api.h("div", { classMap: { "slds-tabs_default__content": true }, key: 1 }, [
      $api.s("", { key: 2 }, [], $slotset),
    ]),
  ])];
}

export function renderTab($api, $cmp, $slotset) {
  const { h, t, s, b } = $api;
  return [h("section", { classMap: { "slds-tabs_default__content": true }, key: 0 }, [
    h("h3", {
      classMap: { "slds-tabs_default__item": true },
      attrs: { role: "tab", tabindex: "0" },
      key: 1,
      on: { click: b($cmp.handleActive) },
    }, [t($cmp.label || "")]),
    s("", { key: 2 }, [], $slotset),
  ])];
}

export function renderSpinner($api, $cmp) {
  return [$api.h("div", { classMap: { "slds-spinner": true }, attrs: { role: "status" }, key: 0 }, [
    $api.t($cmp.alternativeText || "Loading"),
  ])];
}

export function renderIcon($api, $cmp) {
  return [$api.h("span", {
    classMap: { "slds-icon": true },
    attrs: { title: $cmp.alternativeText || $cmp.iconName || "" },
    key: 0,
  }, [$api.t(iconLabel($cmp.iconName))])];
}

export function renderDatatable($api, $cmp) {
  const { h, t, b } = $api;
  const columns = $cmp.columns || [];
  const rows = $cmp.data || [];
  return [h("table", { classMap: { "slds-table": true, "slds-table_cell-buffer": true }, key: 0 }, [
    h("thead", { key: 1 }, [
      h("tr", { key: 2 }, columns.map((column, index) => h("th", { key: 20 + index }, [
        t(column.label || column.fieldName || ""),
      ]))),
    ]),
    h("tbody", { key: 3 }, rows.map((row, rowIndex) => h("tr", { key: 100 + rowIndex }, columns.map((column, colIndex) => {
      if (column.type === "action") {
        const actions = column.typeAttributes?.rowActions || [];
        return h("td", { key: 1000 + rowIndex * 50 + colIndex }, actions.map((action, actionIndex) => h("button", {
          classMap: { "slds-button": true, "slds-button_neutral": true },
          attrs: {
            type: "button",
            "data-row-index": String(rowIndex),
            "data-column-index": String(colIndex),
            "data-action-index": String(actionIndex),
          },
          key: 2000 + rowIndex * 50 + actionIndex,
          on: { click: b($cmp.handleRowAction) },
        }, [t(action.label || action.name || "Action")])));
      }
      const value = row[column.fieldName] ?? "";
      return h("td", { key: 1000 + rowIndex * 50 + colIndex }, [t(value)]);
    })))),
  ])];
}

export function renderRecordForm($api, $cmp, $slotset) {
  const { h, t, s, b } = $api;
  const editMode = $cmp.mode === "edit";
  const children = [
    h("div", { classMap: { "slds-text-title_caps": true }, key: 1 }, [
      t(`${$cmp.objectApiName || "Record"} ${$cmp.recordId || ""}`),
    ]),
    h("div", { key: 2 }, fieldList($cmp.fields).map((field, index) => {
      const name = fieldApiName(field);
      const value = recordFieldDisplayValue($cmp.value, field);
      const fieldChildren = [
        h("span", { classMap: { "slds-form-element__label": true }, key: 200 + index }, [t(name || String(field))]),
      ];
      if (editMode) {
        fieldChildren.push(h("input", {
          classMap: { "slds-input": true },
          attrs: { type: "text", "data-field-name": name },
          props: { value: value || "" },
          key: 400 + index,
        }));
      } else {
        fieldChildren.push(h("div", { classMap: { "slds-form-element__static": true }, key: 400 + index }, [t(value)]));
      }
      return h("div", {
        classMap: { "slds-form-element": true },
        key: 20 + index,
      }, fieldChildren);
    })),
  ];
  if (editMode) {
    children.push(h("button", {
      classMap: { "slds-button": true, "slds-button_brand": true },
      attrs: { type: "submit" },
      key: 3,
    }, [t("Save")]));
  }
  children.push(s("", { key: 4 }, [], $slotset));
  return [h("form", {
    classMap: { "slds-form": true },
    attrs: { "data-object-api-name": $cmp.objectApiName || "" },
    key: 0,
    on: { submit: b($cmp.handleSubmit) },
  }, children)];
}

export function renderOutputField($api, $cmp) {
  const { h, t } = $api;
  return [h("div", { classMap: { "slds-form-element": true }, key: 0 }, [
    h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t($cmp.fieldName || $cmp.name || "")]),
    h("div", { classMap: { "slds-form-element__static": true }, key: 2 }, [t($cmp.value || "")]),
  ])];
}

export function renderMessages($api, _cmp, $slotset) {
  return [$api.h("div", {
    classMap: { "slds-notify": true, "slds-notify_alert": true },
    attrs: { role: "status" },
    key: 0,
  }, [$api.s("", { key: 1 }, [], $slotset)])];
}

export function renderModal($api, $cmp, $slotset) {
  const { h, t, s } = $api;
  return [h("section", {
    classMap: { "slds-modal": true, "slds-fade-in-open": true },
    attrs: { role: "dialog", "aria-modal": "true" },
    key: 0,
  }, [
    h("div", { classMap: { "slds-modal__container": true }, key: 1 }, [
      h("header", { classMap: { "slds-modal__header": true }, key: 2 }, [t($cmp.label || $cmp.title || "")]),
      h("div", { classMap: { "slds-modal__content": true }, key: 3 }, [s("", { key: 4 }, [], $slotset)]),
    ]),
  ])];
}
