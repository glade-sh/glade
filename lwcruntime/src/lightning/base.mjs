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
  "content",
  "href",
  "target",
  "street",
  "city",
  "province",
  "postalCode",
  "country",
  "items",
  "header",
  "placeholder",
  "accept",
  "multiple",
  "flowApiName",
  "flowInputVariables",
  "initials",
  "fallbackIconName",
  "labelWhenOff",
  "labelWhenOn",
  "labelWhenHover",
  "selected",
  "sourceLabel",
  "selectedLabel",
  "min",
  "max",
  "step",
  "mapMarkers",
  "zoomLevel",
  "markersTitle",
  "src",
  "description",
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
      event?.stopPropagation?.();
      const target = event?.target || {};
      this.value = target.value;
      this.checked = Boolean(target.checked);
      this.dispatchEvent(new CustomEvent("change", {
        bubbles: true,
        composed: true,
        detail: { value: target.value, checked: Boolean(target.checked) },
      }));
    }

    handleOptionGroupChange(event) {
      event?.stopPropagation?.();
      const target = event?.target || {};
      if (selector === "lightning-checkbox-group") {
        const values = Array.from(this.template?.querySelectorAll?.('input[type="checkbox"]:checked') || []).map((input) => input.value);
        this.value = values;
        this.dispatchEvent(new CustomEvent("change", {
          bubbles: true,
          composed: true,
          detail: { value: values },
        }));
        return;
      }
      this.value = target.value;
      this.checked = Boolean(target.checked);
      this.dispatchEvent(new CustomEvent("change", {
        bubbles: true,
        composed: true,
        detail: { value: target.value, checked: Boolean(target.checked) },
      }));
    }

    handleDualListboxMove(event) {
      event?.stopPropagation?.();
      const action = event?.currentTarget?.dataset?.action || "";
      const current = selectedValueList(this.value);
      const sourceValues = Array.from(this.template?.querySelector?.('[data-list="source"]')?.selectedOptions || []).map((option) => option.value);
      const selectedValues = Array.from(this.template?.querySelector?.('[data-list="selected"]')?.selectedOptions || []).map((option) => option.value);
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
      this.dispatchEvent(new CustomEvent("change", {
        bubbles: true,
        composed: true,
        detail: { value: values },
      }));
    }

    handleDualListboxChange(event) {
      event?.stopPropagation?.();
      const values = Array.from(event?.target?.selectedOptions || []).map((option) => option.value);
      this.value = values;
      this.dispatchEvent(new CustomEvent("change", {
        bubbles: true,
        composed: true,
        detail: { value: values },
      }));
    }

    handleRichTextChange(event) {
      event?.stopPropagation?.();
      const target = event?.target || {};
      const value = target.value ?? target.innerHTML ?? "";
      this.value = value;
      this.dispatchEvent(new CustomEvent("change", {
        bubbles: true,
        composed: true,
        detail: { value },
      }));
    }

    handleFileUpload(event) {
      const files = Array.from(event?.target?.files || []);
      const uploadedFiles = files.map((file, index) => ({
        name: file.name,
        documentId: `069000000000${String(index + 1).padStart(3, "0")}AAA`,
      }));
      this.dispatchEvent(new CustomEvent("uploadfinished", {
        bubbles: true,
        composed: true,
        detail: { files: uploadedFiles },
      }));
    }

    handleRecordPickerChange(event) {
      event?.stopPropagation?.();
      const target = event?.target || {};
      this.value = target.value;
      this.dispatchEvent(new CustomEvent("change", {
        bubbles: true,
        composed: true,
        detail: { recordId: target.value, value: target.value },
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

export function selectedValueList(value) {
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

export function flattenTreeRows(rows, level = 0) {
  const out = [];
  for (const row of rows || []) {
    out.push({ row, level });
    out.push(...flattenTreeRows(row._children || row.children || row.items || [], level + 1));
  }
  return out;
}

function markerText(marker) {
  const location = marker?.location || {};
  return [
    marker?.title,
    marker?.value,
    location.Name,
    location.City,
    location.State,
    location.Country,
    location.Street,
  ].filter(Boolean).join(", ");
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

export function renderButtonStateful($api, $cmp) {
  const { h, t } = $api;
  const selected = Boolean($cmp.selected || $cmp.checked);
  const label = $cmp.label || (selected ? $cmp.labelWhenOn : $cmp.labelWhenOff) || $cmp.labelWhenHover || "";
  return [h("button", {
    classMap: { "slds-button": true, "slds-button_neutral": true },
    attrs: { type: "button", "aria-pressed": String(selected) },
    props: { disabled: Boolean($cmp.disabled) },
    key: 0,
  }, [t(label)])];
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

export function renderTextContainer(tagName, className) {
  return ($api, $cmp) => [$api.h(tagName, {
    classMap: className ? { [className]: true } : {},
    attrs: { title: String($cmp.value ?? $cmp.label ?? "") },
    key: 0,
  }, [$api.t($cmp.value ?? $cmp.label ?? "")])];
}

export function renderFormattedEmail($api, $cmp) {
  const value = String($cmp.value || $cmp.href || "");
  return [$api.h("a", { attrs: { href: value ? `mailto:${value}` : "#" }, key: 0 }, [
    $api.t($cmp.label || value),
  ])];
}

export function renderSlotContainer(tagName, className) {
  return ($api, _cmp, $slotset) => [$api.h(tagName, {
    classMap: className ? { [className]: true } : {},
    key: 0,
  }, [$api.s("", { key: 1 }, [], $slotset)])];
}

export function renderTitledSlot(tagName, className) {
  return ($api, $cmp, $slotset) => [$api.h(tagName, {
    classMap: className ? { [className]: true } : {},
    key: 0,
  }, [
    $api.h("h3", { key: 1 }, [$api.t($cmp.label || $cmp.name || "")]),
    $api.s("", { key: 2 }, [], $slotset),
  ])];
}

export function renderBreadcrumbs($api, _cmp, $slotset) {
  const { h, s } = $api;
  return [h("nav", { classMap: { "slds-breadcrumb": true }, attrs: { role: "navigation" }, key: 0 }, [
    h("ol", { classMap: { "slds-breadcrumb__list": true }, key: 1 }, [
      s("", { key: 2 }, [], $slotset),
    ]),
  ])];
}

export function renderBreadcrumb($api, $cmp) {
  return [$api.h("a", {
    classMap: { "slds-breadcrumb__item": true },
    attrs: { href: $cmp.href || "#" },
    key: 0,
    on: { click: $api.b($cmp.handleActive) },
  }, [$api.t($cmp.label || $cmp.name || "")])];
}

export function renderFormattedLink(kind) {
  return ($api, $cmp) => {
    const href = kind === "tel" ? `tel:${$cmp.value || ""}` : ($cmp.value || $cmp.href || "#");
    return [$api.h("a", { attrs: { href, target: $cmp.target || undefined }, key: 0 }, [
      $api.t($cmp.label || $cmp.value || $cmp.href || ""),
    ])];
  };
}

export function renderSelect($api, $cmp) {
  const { h, t, b } = $api;
  const value = String($cmp.value ?? "");
  return [h("label", { classMap: { "slds-form-element": true }, key: 0 }, [
    h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t($cmp.label || "")]),
    h("select", {
      classMap: { "slds-select": true },
      props: { value, disabled: Boolean($cmp.disabled) },
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

export function renderSlider($api, $cmp) {
  const { h, t, b } = $api;
  return [h("label", { classMap: { "slds-form-element": true, "slds-slider": true }, key: 0 }, [
    h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t($cmp.label || "")]),
    h("input", {
      classMap: { "slds-slider__range": true },
      attrs: { type: "range", min: $cmp.min ?? "0", max: $cmp.max ?? "100", step: $cmp.step ?? "1" },
      props: { value: $cmp.value ?? "0", disabled: Boolean($cmp.disabled) },
      key: 2,
      on: { change: b($cmp.handleChange), input: b($cmp.handleChange) },
    }),
  ])];
}

export function renderDualListbox($api, $cmp) {
  const { h, t, b } = $api;
  const values = selectedValueList($cmp.value);
  const options = $cmp.options || [];
  const sourceOptions = options.filter((option) => !values.includes(String(option.value ?? option.label ?? "")));
  const selectedOptions = values.map((value) => options.find((option) => String(option.value ?? option.label ?? "") === value) || { label: value, value });
  return [h("fieldset", { classMap: { "slds-form-element": true, "slds-dueling-list": true }, key: 0 }, [
    h("legend", { classMap: { "slds-form-element__legend": true }, key: 1 }, [t($cmp.label || "")]),
    h("div", { classMap: { "slds-dueling-list__column": true }, key: 2 }, [
      h("span", { key: 3 }, [t($cmp.sourceLabel || "Available")]),
      h("select", { classMap: { "slds-select": true }, attrs: { multiple: "", "data-list": "source" }, props: { disabled: Boolean($cmp.disabled) }, key: 4 }, sourceOptions.map((option, index) => {
        const optionValue = String(option.value ?? option.label ?? "");
        return h("option", { attrs: { value: optionValue }, key: 40 + index }, [
          t(option.label || option.value || ""),
        ]);
      })),
    ]),
    h("div", { classMap: { "slds-dueling-list__column": true }, key: 5 }, [
      h("button", {
        classMap: { "slds-button": true, "slds-button_icon": true },
        attrs: { type: "button", "data-action": "add", "aria-label": "Move selection to Selected" },
        props: { disabled: Boolean($cmp.disabled) },
        key: 50,
        on: { click: b($cmp.handleDualListboxMove) },
      }, [t("Move selection to Selected")]),
      h("button", {
        classMap: { "slds-button": true, "slds-button_icon": true },
        attrs: { type: "button", "data-action": "remove", "aria-label": "Move selection to Available" },
        props: { disabled: Boolean($cmp.disabled) },
        key: 51,
        on: { click: b($cmp.handleDualListboxMove) },
      }, [t("Move selection to Available")]),
      h("button", {
        classMap: { "slds-button": true, "slds-button_icon": true },
        attrs: { type: "button", "data-action": "up", "aria-label": "Move selection up" },
        props: { disabled: Boolean($cmp.disabled) },
        key: 52,
        on: { click: b($cmp.handleDualListboxMove) },
      }, [t("Move selection up")]),
      h("button", {
        classMap: { "slds-button": true, "slds-button_icon": true },
        attrs: { type: "button", "data-action": "down", "aria-label": "Move selection down" },
        props: { disabled: Boolean($cmp.disabled) },
        key: 53,
        on: { click: b($cmp.handleDualListboxMove) },
      }, [t("Move selection down")]),
    ]),
    h("div", { classMap: { "slds-dueling-list__column": true }, key: 6 }, [
      h("span", { key: 7 }, [t($cmp.selectedLabel || "Selected")]),
      h("select", { classMap: { "slds-select": true }, attrs: { multiple: "", "data-list": "selected" }, props: { disabled: Boolean($cmp.disabled) }, key: 8 }, selectedOptions.map((option, index) => {
        const optionValue = String(option.value ?? option.label ?? "");
        return h("option", { attrs: { value: optionValue }, key: 80 + index }, [
          t(option.label || option.value || ""),
        ]);
      })),
    ]),
  ])];
}

export function renderInputRichText($api, $cmp) {
  const { h, t, b } = $api;
  return [h("label", { classMap: { "slds-form-element": true }, key: 0 }, [
    h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t($cmp.label || "")]),
    h("textarea", {
      classMap: { "slds-textarea": true, "slds-rich-text-editor__textarea": true },
      props: { value: $cmp.value || "", disabled: Boolean($cmp.disabled) },
      key: 2,
      on: { change: b($cmp.handleRichTextChange), input: b($cmp.handleRichTextChange) },
    }),
  ])];
}

export function renderMenuDivider($api) {
  return [$api.h("div", {
    classMap: { "slds-has-divider_top-space": true },
    attrs: { role: "separator" },
    key: 0,
  })];
}

export function renderProgressBar($api, $cmp) {
  const value = String($cmp.value ?? "0");
  return [$api.h("div", {
    classMap: { "slds-progress-bar": true },
    attrs: { role: "progressbar", "aria-valuemin": "0", "aria-valuemax": "100", "aria-valuenow": value },
    key: 0,
  }, [$api.h("span", { classMap: { "slds-progress-bar__value": true }, key: 1 }, [$api.t(`${value}%`)])])];
}

export function renderProgressRing($api, $cmp) {
  const value = String($cmp.value ?? "0");
  return [$api.h("div", {
    classMap: { "slds-progress-ring": true },
    attrs: { role: "progressbar", "aria-valuenow": value },
    key: 0,
  }, [$api.t(`${value}%`)])];
}

export function renderTile($api, $cmp, $slotset) {
  const { h, t, s } = $api;
  return [h("article", { classMap: { "slds-tile": true }, key: 0 }, [
    h("h3", { classMap: { "slds-tile__title": true }, key: 1 }, [
      h("a", { attrs: { href: $cmp.href || "#" }, key: 2 }, [t($cmp.label || $cmp.title || "")]),
    ]),
    h("div", { classMap: { "slds-tile__detail": true }, key: 3 }, [s("", { key: 4 }, [], $slotset)]),
  ])];
}

export function renderFormattedAddress($api, $cmp) {
  return [$api.h("address", { classMap: { "slds-truncate": true }, key: 0 }, [
    $api.t([$cmp.street, $cmp.city, $cmp.province, $cmp.postalCode, $cmp.country, $cmp.value].filter(Boolean).join(", ")),
  ])];
}

export function renderAvatar($api, $cmp) {
  return [$api.h("span", {
    classMap: { "slds-avatar": true },
    attrs: { title: $cmp.alternativeText || $cmp.label || "" },
    key: 0,
  }, [$api.t(String($cmp.initials || $cmp.fallbackIconName || $cmp.label || "avatar"))])];
}

export function renderHelptext($api, $cmp) {
  return [$api.h("span", {
    classMap: { "slds-form-element__icon": true },
    attrs: { title: $cmp.content || $cmp.label || "" },
    key: 0,
  }, [$api.t($cmp.content || $cmp.label || "?")])];
}

export function renderButtonMenu($api, $cmp, $slotset) {
  const { h, t, s } = $api;
  return [h("div", { classMap: { "slds-dropdown-trigger": true, "slds-dropdown-trigger_click": true }, key: 0 }, [
    h("button", { classMap: { "slds-button": true, "slds-button_neutral": true }, attrs: { type: "button" }, key: 1 }, [t($cmp.label || "Actions")]),
    h("div", { classMap: { "slds-dropdown": true }, key: 2 }, [s("", { key: 3 }, [], $slotset)]),
  ])];
}

export function renderMenuItem($api, $cmp) {
  return [$api.h("button", {
    classMap: { "slds-dropdown__item": true },
    attrs: { type: "button", role: "menuitem" },
    key: 0,
    on: { click: $api.b($cmp.handleActive) },
  }, [$api.t($cmp.label || $cmp.value || "")])];
}

export function renderOptionGroup(inputType) {
  return ($api, $cmp) => {
    const { h, t, b } = $api;
    const values = selectedValueList($cmp.value);
    return [h("fieldset", { classMap: { "slds-form-element": true }, key: 0 }, [
      h("legend", { classMap: { "slds-form-element__legend": true }, key: 1 }, [t($cmp.label || "")]),
      h("div", { classMap: { "slds-form-element__control": true }, key: 2 }, ($cmp.options || []).map((option, index) => {
        const optionValue = String(option.value ?? option.label ?? "");
        return h("label", { classMap: { [`slds-${inputType}`]: true }, key: 20 + index }, [
          h("input", {
            attrs: { type: inputType, value: optionValue, name: $cmp.name || $cmp.label || inputType },
            props: { checked: values.map(String).includes(optionValue) },
            key: 200 + index,
            on: { change: b($cmp.handleOptionGroupChange) },
          }),
          h("span", { key: 400 + index }, [t(option.label || option.value || "")]),
        ]);
      })),
    ])];
  };
}

export function renderInputAddress($api, $cmp) {
  const { h, t, b } = $api;
  return [h("fieldset", { classMap: { "slds-form-element": true }, key: 0 }, [
    h("legend", { classMap: { "slds-form-element__legend": true }, key: 1 }, [t($cmp.label || "Address")]),
    h("input", { classMap: { "slds-input": true }, attrs: { placeholder: "Street" }, props: { value: $cmp.street || "" }, key: 2, on: { change: b($cmp.handleChange) } }),
    h("input", { classMap: { "slds-input": true }, attrs: { placeholder: "City" }, props: { value: $cmp.city || "" }, key: 3, on: { change: b($cmp.handleChange) } }),
  ])];
}

export function renderFileUpload($api, $cmp) {
  const { h, t, b } = $api;
  return [h("label", { classMap: { "slds-form-element": true }, key: 0 }, [
    h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t($cmp.label || "Upload Files")]),
    h("input", {
      classMap: { "slds-file-selector__input": true },
      attrs: { type: "file", accept: $cmp.accept || undefined, multiple: $cmp.multiple ? "" : undefined },
      props: { disabled: Boolean($cmp.disabled) },
      key: 2,
      on: { change: b($cmp.handleFileUpload) },
    }),
  ])];
}

export function renderFlow($api, $cmp) {
  return [$api.h("section", {
    classMap: { "slds-box": true },
    attrs: { "data-flow-api-name": $cmp.flowApiName || "" },
    key: 0,
  }, [$api.t($cmp.flowApiName || $cmp.label || "Flow")])];
}

export function renderPill($api, $cmp) {
  return [$api.h("span", { classMap: { "slds-pill": true }, key: 0 }, [$api.t($cmp.label || $cmp.name || "")])];
}

export function renderPillContainer($api, $cmp, $slotset) {
  const pills = ($cmp.items || []).map((item, index) => $api.h("span", { classMap: { "slds-pill": true }, key: 20 + index }, [
    $api.t(item.label || item.name || item.value || ""),
  ]));
  pills.push($api.s("", { key: 1 }, [], $slotset));
  return [$api.h("div", { classMap: { "slds-pill_container": true }, key: 0 }, pills)];
}

export function renderQuickActionPanel($api, $cmp, $slotset) {
  const { h, t, s } = $api;
  return [h("section", { classMap: { "slds-modal": true, "slds-fade-in-open": true }, attrs: { role: "dialog" }, key: 0 }, [
    h("div", { classMap: { "slds-modal__container": true }, key: 1 }, [
      h("header", { classMap: { "slds-modal__header": true }, key: 2 }, [t($cmp.header || $cmp.title || "")]),
      h("div", { classMap: { "slds-modal__content": true }, key: 3 }, [s("", { key: 4 }, [], $slotset)]),
      h("footer", { classMap: { "slds-modal__footer": true }, key: 5 }, [s("footer", { key: 6 }, [], $slotset)]),
    ]),
  ])];
}

export function renderRecordPicker($api, $cmp) {
  const { h, t, b } = $api;
  return [h("label", { classMap: { "slds-form-element": true }, key: 0 }, [
    h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t($cmp.label || "")]),
    h("input", {
      classMap: { "slds-input": true },
      attrs: { placeholder: $cmp.placeholder || "Search records" },
      props: { value: $cmp.value || "" },
      key: 2,
      on: { change: b($cmp.handleRecordPickerChange), input: b($cmp.handleRecordPickerChange) },
    }),
  ])];
}

export function renderTree($api, $cmp, $slotset) {
  const children = ($cmp.items || []).map((item, index) => $api.h("li", { attrs: { role: "treeitem" }, key: 20 + index }, [
    $api.t(item.label || item.name || ""),
  ]));
  children.push($api.s("", { key: 1 }, [], $slotset));
  return [$api.h("ul", { classMap: { "slds-tree": true }, attrs: { role: "tree" }, key: 0 }, children)];
}

export function renderTreeGrid($api, $cmp) {
  const { h, t } = $api;
  const columns = $cmp.columns || [];
  const rows = flattenTreeRows($cmp.data || []);
  return [h("table", { classMap: { "slds-table": true, "slds-tree": true }, attrs: { role: "treegrid" }, key: 0 }, [
    h("thead", { key: 1 }, [
      h("tr", { key: 2 }, columns.map((column, index) => h("th", { key: 20 + index }, [t(column.label || column.fieldName || "")]))),
    ]),
    h("tbody", { key: 3 }, rows.map((entry, rowIndex) => h("tr", { attrs: { "aria-level": String(entry.level + 1) }, key: 100 + rowIndex }, columns.map((column, colIndex) => {
      const value = entry.row?.[column.fieldName] ?? "";
      const prefix = colIndex === 0 ? `${"  ".repeat(entry.level)}` : "";
      return h("td", { key: 1000 + rowIndex * 50 + colIndex }, [t(prefix + value)]);
    })))),
  ])];
}

export function renderMap($api, $cmp) {
  const markers = $cmp.mapMarkers || $cmp.markers || $cmp.items || [];
  return [$api.h("section", { classMap: { "slds-map": true }, attrs: { "data-zoom-level": String($cmp.zoomLevel || "") }, key: 0 }, [
    $api.h("h3", { key: 1 }, [$api.t($cmp.markersTitle || $cmp.title || "Map")]),
    $api.h("ul", { key: 2 }, markers.map((marker, index) => $api.h("li", { key: 20 + index }, [
      $api.t(markerText(marker)),
    ]))),
  ])];
}

export function renderCarousel($api, _cmp, $slotset) {
  return [$api.h("section", { classMap: { "slds-carousel": true }, key: 0 }, [
    $api.s("", { key: 1 }, [], $slotset),
  ])];
}

export function renderCarouselImage($api, $cmp) {
  const { h, t } = $api;
  return [h("figure", { classMap: { "slds-carousel__panel": true }, key: 0 }, [
    h("img", { attrs: { src: $cmp.src || "", alt: $cmp.alternativeText || $cmp.header || "" }, key: 1 }),
    h("figcaption", { key: 2 }, [
      h("h3", { key: 3 }, [t($cmp.header || $cmp.label || "")]),
      h("p", { key: 4 }, [t($cmp.description || "")]),
    ]),
  ])];
}

export function renderVerticalNavigationItem($api, $cmp) {
  return [$api.h("a", {
    classMap: { "slds-nav-vertical__action": true },
    attrs: { href: $cmp.href || "#", role: "link" },
    key: 0,
    on: { click: $api.b($cmp.handleActive) },
  }, [$api.t($cmp.label || $cmp.name || "")])];
}
