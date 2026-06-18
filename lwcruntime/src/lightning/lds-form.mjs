import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import { reportDiagnostic } from "@glade/shell/diagnostics";

const COMMON_PROPS = [
  "label",
  "value",
  "checked",
  "disabled",
  "required",
  "type",
  "name",
  "fieldName",
  "options",
  "objectApiName",
  "recordId",
  "fields",
  "mode",
  "columns",
  "data",
  "keyField",
  "draftValues",
  "selectedRows",
  "hideCheckboxColumn",
  "sortedBy",
  "sortedDirection",
  "enableInfiniteLoading",
  "error",
  "dirty",
];

const UNSUPPORTED_ATTRIBUTE_NAMES = new Set([
  "hide-checkbox-column",
  "max-row-selection",
  "sorted-by",
  "sorted-direction",
  "show-row-number-column",
  "wrap-text-max-lines",
]);

const COMMON_METHODS = [
  "setErrors",
  "getErrors",
  "wireRecordUi",
  "getWiredData",
  "wirePicklistValues",
  "getWiredPicklistValues",
  "setValue",
  "clean",
  "reset",
  "setCustomValidity",
  "checkValidity",
  "reportValidity",
  "focus",
  "blur",
  "getSelectedRows",
  "submit",
];

function publicProps() {
  const props = {};
  for (const name of COMMON_PROPS) props[name] = { config: 0 };
  return props;
}

export function createDatatable() {
  return createComponent("lightning-datatable", renderDatatable);
}

export function createInputField() {
  return createComponent("lightning-input-field", renderInputField);
}

export function createOutputField() {
  return createComponent("lightning-output-field", renderOutputField);
}

export function createMessages() {
  return createComponent("lightning-messages", renderMessages);
}

export function createRecordForm(selector) {
  return createComponent(selector, renderRecordForm);
}

function createComponent(selector, render) {
  class LocalBaseComponent extends LightningElement {
    connectedCallback() {
      this.__selector = selector;
      if (this.__initialValue === undefined) this.__initialValue = this.value;
      this.reportUnsupportedAttributes();
      if (selector === "lightning-input-field") registerInputFieldWithNearestForm(this);
      if (isRecordFormSelector(selector)) this.loadRecord();
    }

    reportUnsupportedAttributes() {
      const unsupportedAttrs = unsupportedBaseAttributes(this);
      if (!unsupportedAttrs.length) return;
      const message = `GLADELWC061 base component attributes unsupported locally: ${unsupportedAttrs.join(", ")}`;
      reportDiagnostic({ code: "GLADELWC061", severity: "warning", message, tagName: selector, attributes: unsupportedAttrs });
    }

    handleChange(event) {
      event?.stopPropagation?.();
      const target = event?.target || {};
      this.value = target.type === "checkbox" ? Boolean(target.checked) : target.value;
      this.checked = Boolean(target.checked);
      const detail = { value: this.value, checked: Boolean(target.checked) };
      const fieldName = normalizeFieldName(this.fieldName || this.name);
      if (fieldName) detail.fieldName = fieldName;
      this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail }));
    }

    handleFormFieldChange(event) {
      const fieldName = normalizeFieldName(event?.detail?.fieldName || event?.target?.fieldName || event?.target?.dataset?.fieldName);
      if (!fieldName) return;
      this.__formFieldValues = { ...(this.__formFieldValues || {}), [fieldName]: event?.detail?.value ?? event?.target?.value };
    }

    handleSubmit(event) {
      event?.preventDefault?.();
      if (!this.reportFormValidity()) {
        const detail = { message: "Review the fields with errors." };
        this.error = detail;
        this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail }));
        return;
      }
      const fields = this.collectRecordFormFields();
      const submitEvent = new CustomEvent("submit", {
        bubbles: true,
        composed: true,
        cancelable: true,
        detail: { fields },
      });
      this.dispatchEvent(submitEvent);
      if (!submitEvent.defaultPrevented) this.saveRecord(fields);
    }

    submit(fields) {
      const submitFields = { ...(fields || this.collectRecordFormFields()) };
      const submitEvent = new CustomEvent("submit", {
        bubbles: true,
        composed: true,
        cancelable: true,
        detail: { fields: submitFields },
      });
      this.dispatchEvent(submitEvent);
      if (!submitEvent.defaultPrevented) this.saveRecord(submitFields);
    }

    handleCancel(event) {
      event?.preventDefault?.();
      const fields = this.collectRecordFormFields();
      for (const field of inputFieldsForForm(this)) field.reset?.();
      this.dispatchEvent(new CustomEvent("cancel", { bubbles: true, composed: true, detail: { fields } }));
    }

    collectRecordFormFields() {
      return { ...(this.__formFieldValues || {}), ...collectFormFields(this) };
    }

    reportFormValidity() {
      let valid = true;
      for (const field of inputFieldsForForm(this)) {
        if (field.reportValidity && !field.reportValidity()) valid = false;
      }
      for (const control of this.template?.querySelectorAll?.("[data-field-name]") || []) {
        if (control.reportValidity && !control.reportValidity()) valid = false;
      }
      return valid;
    }

    loadRecord() {
      if (!this.objectApiName || this.__recordLoaded) return;
      if (!this.recordId && this.__selector === "lightning-record-edit-form") {
        this.loadCreateDefaults();
        return;
      }
      if (!this.recordId) return;
      this.__recordLoaded = true;
      fetch("/lightning/wire/getRecord", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          recordId: this.recordId,
          fields: recordFieldRefs(this.objectApiName, this.fields),
        }),
      }).then((response) => response.json()).then((result) => {
        if (result?.error) {
          this.error = result.error;
          this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail: result.error }));
          return;
        }
        this.value = result?.data || result;
        this.dispatchEvent(new CustomEvent("load", {
          bubbles: true,
          composed: true,
          detail: {
            record: this.value,
            records: this.value?.id ? { [this.value.id]: this.value } : {},
            objectInfos: result?.data?.objectInfos || {},
          },
        }));
      }).catch((err) => {
        const detail = { message: err?.message || String(err) };
        this.error = detail;
        this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail }));
      });
    }

    loadCreateDefaults() {
      if (this.__recordLoaded) return;
      this.__recordLoaded = true;
      fetch("/lightning/wire/getRecordCreateDefaults", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          objectApiName: this.objectApiName,
          fields: recordFieldRefs(this.objectApiName, this.fields),
        }),
      }).then((response) => response.json()).then((result) => {
        if (result?.error) {
          this.error = result.error;
          this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail: result.error }));
          return;
        }
        const data = result?.data || result;
        this.value = data?.record || data;
        applyCreateDefaultsToFields(this, data);
        this.dispatchEvent(new CustomEvent("load", {
          bubbles: true,
          composed: true,
          detail: {
            record: this.value,
            records: this.value?.id ? { [this.value.id]: this.value } : {},
            objectInfos: data?.objectInfos || {},
            layout: data?.layout,
          },
        }));
      }).catch((err) => {
        const detail = { message: err?.message || String(err) };
        this.error = detail;
        this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail }));
      });
    }

    saveRecord(fields) {
      const endpoint = this.recordId ? "/lightning/wire/updateRecord" : "/lightning/wire/createRecord";
      const body = this.recordId
        ? { fields: { Id: this.recordId, ...fields } }
        : { apiName: this.objectApiName, objectApiName: this.objectApiName, fields };
      fetch(endpoint, {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(body),
      }).then((response) => response.json()).then((result) => {
        if (result?.error) {
          this.error = result.error;
          this.dispatchEvent(new CustomEvent("error", { bubbles: true, composed: true, detail: result.error }));
          return;
        }
        if (result?.data) this.value = result.data;
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
      const control = formControl(this);
      if (control && "value" in control) control.value = this.value ?? "";
      if (control && "checked" in control) control.checked = Boolean(this.value || this.checked);
      control?.setCustomValidity?.("");
    }

    setCustomValidity(message) {
      this.__customValidityMessage = String(message || "");
      formControl(this)?.setCustomValidity?.(this.__customValidityMessage);
    }

    checkValidity() {
      const control = formControl(this);
      syncControlValue(control, this);
      const message = validityMessage({ required: this.required, value: this.value, customError: this.__customValidityMessage, type: inputTypeForField(this) });
      control?.setCustomValidity?.(message || "");
      if (message) return false;
      return control?.checkValidity ? control.checkValidity() : true;
    }

    reportValidity() {
      const control = formControl(this);
      syncControlValue(control, this);
      const message = validityMessage({ required: this.required, value: this.value, customError: this.__customValidityMessage, type: inputTypeForField(this) });
      control?.setCustomValidity?.(message || "");
      if (message) {
        control?.reportValidity?.();
        return false;
      }
      return control?.reportValidity ? control.reportValidity() : true;
    }

    focus() {
      formControl(this)?.focus?.();
    }

    blur() {
      formControl(this)?.blur?.();
    }

    getSelectedRows() {
      const keyField = this.keyField || "id";
      const selected = new Set((this.selectedRows || []).map(String));
      return (this.data || []).filter((row) => selected.has(String(row?.[keyField])));
    }

    handleRowSelection(event) {
      const key = event?.currentTarget?.dataset?.rowKey || "";
      const selected = new Set((this.selectedRows || []).map(String));
      if (event?.currentTarget?.checked) selected.add(key);
      else selected.delete(key);
      this.selectedRows = Array.from(selected);
      this.dispatchEvent(new CustomEvent("rowselection", {
        bubbles: true,
        composed: true,
        detail: { selectedRows: this.getSelectedRows(), selectedRowKeys: this.selectedRows },
      }));
    }

    handleSort(event) {
      const fieldName = event?.currentTarget?.dataset?.fieldName || "";
      if (!fieldName) return;
      const sortDirection = this.sortedBy === fieldName && this.sortedDirection === "asc" ? "desc" : "asc";
      this.sortedBy = fieldName;
      this.sortedDirection = sortDirection;
      this.dispatchEvent(new CustomEvent("sort", {
        bubbles: true,
        composed: true,
        detail: { fieldName, sortedBy: fieldName, sortDirection },
      }));
    }

    handleCellChange(event) {
      const dataset = event?.currentTarget?.dataset || {};
      const rowKey = dataset.rowKey || "";
      const fieldName = dataset.fieldName || "";
      if (!fieldName) return;
      const value = event?.currentTarget?.type === "checkbox" ? Boolean(event.currentTarget.checked) : event?.currentTarget?.value;
      const keyField = this.keyField || "id";
      const drafts = Array.isArray(this.draftValues) ? this.draftValues.map((draft) => ({ ...draft })) : [];
      const existing = drafts.find((draft) => String(draft[keyField]) === String(rowKey));
      if (existing) existing[fieldName] = value;
      else drafts.push({ [keyField]: rowKey, [fieldName]: value });
      this.draftValues = drafts;
      this.dispatchEvent(new CustomEvent("cellchange", { bubbles: true, composed: true, detail: { draftValues: drafts } }));
    }

    handleDatatableSave() {
      const draftValues = Array.isArray(this.draftValues) ? this.draftValues.slice() : [];
      this.dispatchEvent(new CustomEvent("save", { bubbles: true, composed: true, detail: { draftValues } }));
    }

    handleDatatableCancel() {
      this.draftValues = [];
      this.dispatchEvent(new CustomEvent("cancel", { bubbles: true, composed: true, detail: { draftValues: [] } }));
    }

    handleLoadMore() {
      this.dispatchEvent(new CustomEvent("loadmore", { bubbles: true, composed: true, detail: {} }));
    }

    handleRowAction(event) {
      const dataset = event?.currentTarget?.dataset || {};
      const rowIndex = Number(dataset.rowIndex);
      const columnIndex = Number(dataset.columnIndex);
      const actionIndex = Number(dataset.actionIndex);
      const row = (this.data || [])[rowIndex];
      const column = (this.columns || [])[columnIndex];
      const actions = column?.type === "button"
        ? [{ label: attrValue(column?.typeAttributes?.label, row) || column?.label || "Action", name: attrValue(column?.typeAttributes?.name, row) || column?.fieldName || "action" }]
        : (column?.typeAttributes?.rowActions || []);
      const action = actions[actionIndex];
      if (!row || !action) return;
      this.dispatchEvent(new CustomEvent("rowaction", { bubbles: true, composed: true, detail: { action, row } }));
    }
  }

  registerDecorators(LocalBaseComponent, { publicProps: publicProps(), publicMethods: COMMON_METHODS });
  render.stylesheets = [];
  render.slots = [""];
  const template = registerTemplate(render);
  freezeTemplate(render);
  return registerComponent(LocalBaseComponent, { tmpl: template, sel: selector, apiVersion: 63 });
}

function renderInputField($api, $cmp) {
  const { h, t, b } = $api;
  const options = $cmp.options || picklistValuesForField($cmp, $cmp.fieldName);
  if (options.length) {
    return [h("label", { classMap: { "slds-form-element": true }, key: 0 }, [
      h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t($cmp.label || normalizeFieldName($cmp.fieldName) || "")]),
      h("select", {
        classMap: { "slds-select": true },
        attrs: { "data-field-name": normalizeFieldName($cmp.fieldName) || undefined },
        props: { value: String($cmp.value ?? ""), disabled: Boolean($cmp.disabled), required: Boolean($cmp.required) },
        key: 2,
        on: { change: b($cmp.handleChange), input: b($cmp.handleChange) },
      }, options.map((option, index) => {
        const optionValue = String(option.value ?? option.label ?? "");
        return h("option", { attrs: { value: optionValue }, props: { selected: optionValue === String($cmp.value ?? "") }, key: 20 + index }, [
          t(option.label || option.value || ""),
        ]);
      })),
    ])];
  }
  const type = inputTypeForField($cmp);
  return [h("label", { classMap: { "slds-form-element": true }, key: 0 }, [
    h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t($cmp.label || normalizeFieldName($cmp.fieldName) || "")]),
    h("input", {
      classMap: { "slds-input": true },
      attrs: { type, step: type === "number" ? "any" : undefined, "data-field-name": normalizeFieldName($cmp.fieldName) || undefined },
      props: {
        value: type === "checkbox" ? undefined : ($cmp.value ?? ""),
        checked: type === "checkbox" ? Boolean($cmp.value || $cmp.checked) : undefined,
        disabled: Boolean($cmp.disabled),
        required: Boolean($cmp.required),
      },
      key: 2,
      on: { change: b($cmp.handleChange), input: b($cmp.handleChange) },
    }),
  ])];
}

function renderOutputField($api, $cmp) {
  const { h, t } = $api;
  const value = $cmp.value ?? readRecordDisplayValue($cmp.record || $cmp.value, $cmp.fieldName || $cmp.name);
  return [h("div", { classMap: { "slds-form-element": true }, key: 0 }, [
    h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t($cmp.fieldName || $cmp.name || "")]),
    h("div", { classMap: { "slds-form-element__static": true }, key: 2 }, [t(value || "")]),
  ])];
}

function renderMessages($api, _cmp, $slotset) {
  return [$api.h("div", { classMap: { "slds-notify": true, "slds-notify_alert": true }, attrs: { role: "status" }, key: 0 }, [
    $api.s("", { key: 1 }, [], $slotset),
  ])];
}

function renderRecordForm($api, $cmp, $slotset) {
  const { h, t, s, b } = $api;
  const editMode = $cmp.mode === "edit" || $cmp.__selector === "lightning-record-edit-form";
  const children = [
    h("div", { classMap: { "slds-text-title_caps": true }, key: 1 }, [t(`${$cmp.objectApiName || "Record"} ${$cmp.recordId || ""}`)]),
    h("div", { key: 2 }, fieldList($cmp.fields).map((field, index) => {
      const name = normalizeFieldName(field);
      const value = readRecordDisplayValue($cmp.value, field);
      const fieldChildren = [h("span", { classMap: { "slds-form-element__label": true }, key: 200 + index }, [t(name || String(field))])];
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
      return h("div", { classMap: { "slds-form-element": true }, key: 20 + index }, fieldChildren);
    })),
  ];
  if (editMode) {
    children.push(h("button", { classMap: { "slds-button": true, "slds-button_brand": true }, attrs: { type: "submit" }, key: 3 }, [t("Save")]));
    children.push(h("button", { classMap: { "slds-button": true, "slds-button_neutral": true }, attrs: { type: "button" }, key: 5, on: { click: b($cmp.handleCancel) } }, [t("Cancel")]));
  }
  children.push(s("", { key: 4 }, [], $slotset));
  return [h("form", {
    classMap: { "slds-form": true },
    attrs: { "data-object-api-name": $cmp.objectApiName || "" },
    key: 0,
    on: { submit: b($cmp.handleSubmit), change: b($cmp.handleFormFieldChange) },
  }, children)];
}

function renderDatatable($api, $cmp) {
  const { h, t, b } = $api;
  const columns = $cmp.columns || [];
  const rows = $cmp.data || [];
  const keyField = $cmp.keyField || "id";
  const selected = new Set(($cmp.selectedRows || []).map(String));
  const hasEditable = columns.some((column) => column.editable);
  const tableChildren = [
    h("table", { classMap: { "slds-table": true, "slds-table_cell-buffer": true, "slds-table_bordered": true }, key: 0 }, [
      h("thead", { key: 1 }, [
        h("tr", { key: 2 }, [
          ...($cmp.hideCheckboxColumn ? [] : [h("th", { key: 10 }, [t("Select")])]),
          ...columns.map((column, index) => h("th", { key: 20 + index }, [
            column.sortable
              ? h("button", { classMap: { "slds-button": true, "slds-button_reset": true }, attrs: { type: "button", "data-field-name": column.fieldName || "" }, key: 200 + index, on: { click: b($cmp.handleSort) } }, [t(column.label || column.fieldName || "")])
              : t(column.label || column.fieldName || ""),
          ])),
        ]),
      ]),
      h("tbody", { key: 3 }, rows.map((row, rowIndex) => {
        const rowKey = String(row?.[keyField] ?? rowIndex);
        return h("tr", { attrs: { "data-row-key": rowKey }, key: stableRowKey(row, keyField, rowIndex) }, [
          ...($cmp.hideCheckboxColumn ? [] : [h("td", { key: 900 + rowIndex }, [
            h("input", { attrs: { type: "checkbox", "data-row-key": rowKey, "aria-label": `Select row ${rowIndex + 1}` }, props: { checked: selected.has(rowKey) }, key: 901 + rowIndex, on: { change: b($cmp.handleRowSelection) } }),
          ])]),
          ...columns.map((column, colIndex) => h("td", { key: 1000 + rowIndex * 50 + colIndex }, datatableCell($api, $cmp, row, rowIndex, column, colIndex, rowKey))),
        ]);
      })),
    ]),
  ];
  if (hasEditable) {
    tableChildren.push(h("div", { classMap: { "slds-m-top_x-small": true }, key: 4 }, [
      h("button", { classMap: { "slds-button": true, "slds-button_brand": true }, attrs: { type: "button" }, key: 40, on: { click: b($cmp.handleDatatableSave) } }, [t("Save")]),
      h("button", { classMap: { "slds-button": true, "slds-button_neutral": true }, attrs: { type: "button" }, key: 41, on: { click: b($cmp.handleDatatableCancel) } }, [t("Cancel")]),
    ]));
  }
  tableChildren.push(h("button", { classMap: { "slds-button": true, "slds-button_neutral": true }, attrs: { type: "button" }, key: 5, on: { click: b($cmp.handleLoadMore) } }, [t("Load More")]));
  return [h("div", { classMap: { "slds-table_edit_container": true }, key: 9 }, tableChildren)];
}

function datatableCell($api, $cmp, row, rowIndex, column, colIndex, rowKey) {
  const { h, t, b } = $api;
  const type = String(column.type || "text").toLowerCase();
  if (type === "action") {
    return (column.typeAttributes?.rowActions || []).map((action, actionIndex) => h("button", {
      classMap: { "slds-button": true, "slds-button_neutral": true },
      attrs: { type: "button", "data-row-index": String(rowIndex), "data-column-index": String(colIndex), "data-action-index": String(actionIndex) },
      key: 2000 + rowIndex * 50 + actionIndex,
      on: { click: b($cmp.handleRowAction) },
    }, [t(action.label || action.name || "Action")]));
  }
  if (type === "button") {
    const attrs = column.typeAttributes || {};
    const label = attrValue(attrs.label, row) || column.label || "Action";
    return [h("button", {
      classMap: { "slds-button": true, "slds-button_neutral": true },
      attrs: { type: "button", "data-row-index": String(rowIndex), "data-column-index": String(colIndex), "data-action-index": "0" },
      key: 2100 + rowIndex * 50 + colIndex,
      on: { click: b($cmp.handleRowAction) },
    }, [t(label)])];
  }
  const raw = row?.[column.fieldName];
  if (column.editable && (type === "text" || type === "string")) {
    return [
      h("span", { classMap: { "slds-m-right_x-small": true }, key: 2199 + rowIndex * 50 + colIndex }, [t(raw ?? "")]),
      h("input", { classMap: { "slds-input": true }, attrs: { type: "text", "data-row-key": rowKey, "data-field-name": column.fieldName || "" }, props: { value: raw ?? "" }, key: 2200 + rowIndex * 50 + colIndex, on: { change: b($cmp.handleCellChange) } }),
    ];
  }
  if (type === "boolean") return [h("input", { attrs: { type: "checkbox", disabled: "" }, props: { checked: Boolean(raw) }, key: 2300 + rowIndex * 50 + colIndex })];
  if (type === "url") return [h("a", { attrs: { href: raw || "#", target: column.typeAttributes?.target || undefined }, key: 2400 + rowIndex * 50 + colIndex }, [t(attrValue(column.typeAttributes?.label, row) || raw || "")])];
  if (type === "email") return [h("a", { attrs: { href: raw ? `mailto:${raw}` : "#" }, key: 2500 + rowIndex * 50 + colIndex }, [t(raw || "")])];
  if (type === "phone") return [h("a", { attrs: { href: raw ? `tel:${raw}` : "#" }, key: 2600 + rowIndex * 50 + colIndex }, [t(raw || "")])];
  if (type === "badge") return [h("span", { classMap: { "slds-badge": true }, key: 2700 + rowIndex * 50 + colIndex }, [t(raw ?? "")])];
  return [t(formatDatatableValue(raw, type, column))];
}

export function normalizeFieldName(fieldName) {
  if (fieldName && typeof fieldName === "object") return fieldName.fieldApiName || fieldName.apiName || fieldName.fieldName || fieldName.name || String(fieldName);
  const parts = String(fieldName || "").split(".");
  return parts[parts.length - 1] || "";
}

export function readRecordDisplayValue(record, fieldName) {
  const field = record?.fields?.[normalizeFieldName(fieldName)];
  if (field && typeof field === "object") return field.displayValue ?? field.value ?? "";
  return field ?? "";
}

export function collectFormFields(root) {
  const fields = {};
  for (const field of inputFieldsForForm(root)) {
    const name = normalizeFieldName(field.fieldName || field.name);
    if (name) fields[name] = field.value;
  }
  for (const control of root?.template?.querySelectorAll?.("[data-field-name]") || []) {
    const name = normalizeFieldName(control.dataset?.fieldName);
    if (name) fields[name] = control.type === "checkbox" ? Boolean(control.checked) : control.value;
  }
  return fields;
}

function syncControlValue(control, component) {
  if (!control) return;
  const type = inputTypeForField(component);
  if (type === "checkbox" && "checked" in control) control.checked = Boolean(component.value || component.checked);
  else if ("value" in control) control.value = component.value ?? "";
}

export function validityMessage({ required, value, customError, type }) {
  if (customError) return customError;
  if (required && type === "checkbox" && !value) return "Complete this field.";
  if (required && (value === "" || value === null || value === undefined)) return "Complete this field.";
  return "";
}

function inputFieldsForForm(root) {
  const fields = [
    ...(root?.querySelectorAll?.("lightning-input-field") || []),
    ...(root?.hostElement?.__gladeInputFields || []),
    ...(root?.template?.host?.__gladeInputFields || []),
  ];
  for (const container of [root, root?.hostElement, root?.template?.host]) {
    for (const element of container?.children || []) collectAssignedInputFields(element, fields);
  }
  for (const slot of root?.template?.querySelectorAll?.("slot") || []) {
    for (const element of slot.assignedElements?.({ flatten: true }) || []) collectAssignedInputFields(element, fields);
  }
  return [...new Set(fields)];
}

function collectAssignedInputFields(element, fields) {
  if (!element) return;
  if (element.tagName?.toLowerCase() === "lightning-input-field") fields.push(element);
  for (const child of element.querySelectorAll?.("lightning-input-field") || []) fields.push(child);
}

function registerInputFieldWithNearestForm(component) {
  const host = component?.hostElement || component?.template?.host;
  let node = host?.parentElement || null;
  while (node) {
    const tag = node.tagName?.toLowerCase();
    if (tag === "lightning-record-edit-form" || tag === "lightning-record-form" || tag === "lightning-record-view-form") {
      node.__gladeInputFields = node.__gladeInputFields || [];
      if (!node.__gladeInputFields.includes(component)) node.__gladeInputFields.push(component);
      return;
    }
    const root = node.getRootNode?.();
    node = node.parentElement || root?.host?.parentElement || null;
  }
}

function applyCreateDefaultsToFields(form, data) {
  const objectInfo = data?.objectInfos?.[form.objectApiName] || {};
  const metadataFields = objectInfo.fields || {};
  const defaultFields = data?.record?.fields || {};
  for (const field of inputFieldsForForm(form)) {
    const name = normalizeFieldName(field.fieldName || field.name);
    if (!name) continue;
    const metadata = metadataFields[name] || {};
    const defaultField = defaultFields[name];
    const defaultValue = defaultField && typeof defaultField === "object" ? defaultField.value : defaultField;
    if (defaultValue !== undefined && field.value === undefined) {
      field.value = defaultValue;
      field.__initialValue = defaultValue;
    }
    if (!field.label && metadata.label) field.label = metadata.label;
    if (field.required === undefined && metadata.required !== undefined) field.required = Boolean(metadata.required);
    if (!field.type) {
      const inputType = inputTypeForMetadata(metadata);
      if (inputType) field.type = inputType;
    }
    if (!field.options && Array.isArray(metadata.picklistValues)) {
      field.options = metadata.picklistValues.map((option) => ({
        label: option.label ?? option.value ?? "",
        value: option.value ?? option.label ?? "",
      }));
    }
  }
}

function formControl(component) {
  return component?.template?.querySelector?.("input, textarea, select, button, [tabindex]") || null;
}

function unsupportedBaseAttributes(component) {
  const host = component?.hostElement || component;
  if (!host || typeof host.getAttributeNames !== "function") return [];
  const unsupportedAttrs = [];
  for (const name of host.getAttributeNames()) {
    if (UNSUPPORTED_ATTRIBUTE_NAMES.has(name)) unsupportedAttrs.push(name);
  }
  return unsupportedAttrs;
}

function fieldList(fields) {
  if (Array.isArray(fields)) return fields;
  if (typeof fields === "string" && fields.trim()) return fields.split(",").map((field) => field.trim());
  return [];
}

function recordFieldRefs(objectApiName, fields) {
  return fieldList(fields).map((field) => {
    const raw = typeof field === "string" ? field : (field?.fieldApiName || field?.apiName || field?.fieldName || field?.name || "");
    if (!raw) return "";
    return String(raw).includes(".") ? String(raw) : `${objectApiName}.${raw}`;
  }).filter(Boolean);
}

function isRecordFormSelector(selector) {
  return selector === "lightning-record-form" || selector === "lightning-record-view-form" || selector === "lightning-record-edit-form";
}

function inputTypeForField(component) {
  const type = String(component?.type || component?.dataType || "").toLowerCase();
  if (["checkbox", "boolean"].includes(type)) return "checkbox";
  if (["number", "double", "integer", "currency", "percent"].includes(type)) return "number";
  if (type === "date") return "date";
  if (["datetime", "datetime-local"].includes(type)) return "datetime-local";
  if (type === "email") return "email";
  if (["phone", "tel"].includes(type)) return "tel";
  return "text";
}

function inputTypeForMetadata(metadata) {
  const type = String(metadata?.dataType || metadata?.type || "").toLowerCase();
  if (["picklist", "multipicklist"].includes(type)) return "";
  if (["boolean", "checkbox"].includes(type)) return "checkbox";
  if (["double", "integer", "currency", "percent", "number"].includes(type)) return "number";
  if (type === "date") return "date";
  if (["datetime", "datetime-local"].includes(type)) return "datetime-local";
  if (type === "email") return "email";
  if (["phone", "tel"].includes(type)) return "tel";
  return "";
}

function picklistValuesForField(component, fieldName) {
  const name = normalizeFieldName(fieldName);
  const wired = component?.getWiredPicklistValues?.() || component?.__wiredPicklistValues || {};
  const direct = wired?.[name]?.values || wired?.values;
  return Array.isArray(direct) ? direct : [];
}

function stableRowKey(row, keyField, rowIndex) {
  const value = row?.[keyField];
  return value === undefined || value === null || value === "" ? 100 + rowIndex : String(value);
}

function attrValue(value, row) {
  if (value && typeof value === "object" && value.fieldName) return row?.[value.fieldName];
  return value;
}

function formatDatatableValue(value, type, column) {
  if (value === undefined || value === null) return "";
  if (type === "currency") {
    try {
      return new Intl.NumberFormat(undefined, { style: "currency", currency: column.typeAttributes?.currencyCode || "USD" }).format(Number(value));
    } catch {
      return String(value);
    }
  }
  if (type === "number") {
    try {
      return new Intl.NumberFormat().format(Number(value));
    } catch {
      return String(value);
    }
  }
  if (type === "date" || type === "date-local") {
    const date = new Date(value);
    if (!Number.isNaN(date.getTime())) return date.toISOString().slice(0, 10);
  }
  return String(value);
}
