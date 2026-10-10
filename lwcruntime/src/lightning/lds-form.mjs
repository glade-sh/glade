import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import { reportDiagnostic } from "@glade/shell/diagnostics";
import { createRecord, updateRecord, getRecord } from "lightning/uiRecordApi";
import { getObjectInfo } from "lightning/uiObjectInfoApi";
import locale from "@salesforce/i18n/locale";
import currency from "@salesforce/i18n/currency";

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

let datatableInstance = 0;
// Remember successful defaults hydration in this page's module realm. A new
// edit-form publishes the cached defaults and their hydrated fields separately;
// the request still runs so record defaults and metadata cannot become stale.
const hydratedCreateDefaults = new Set();

function publicProps(selector) {
  const props = {};
  for (const name of COMMON_PROPS) props[name] = { config: 0 };
  if (selector === "lightning-datatable") {
    for (const name of ["maxRowSelection", "disabledRows", "showRowNumberColumn"]) props[name] = { config: 0 };
    for (const name of ["selectedRows", "draftValues"]) props[name] = { config: 3 };
  }
  if (isRecordFormSelector(selector) || isFieldSelector(selector)) {
    for (const name of ["readOnly", "variant", "density", "layoutType", "editable"]) props[name] = { config: 0 };
  }
  if (isFieldSelector(selector)) {
    props.value = { config: 3 };
    props.required = { config: 3 };
  }
  return props;
}

export function createDatatable() {
  return createComponent("lightning-datatable", renderDatatable);
}

let inputFieldClass, outputFieldClass;

export function createInputField() {
  return inputFieldClass ||= createComponent("lightning-input-field", renderInputField);
}

export function createOutputField() {
  return outputFieldClass ||= createComponent("lightning-output-field", renderOutputField);
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
      if (selector === "lightning-datatable") {
        this.__datatableID = "glade-datatable-" + (++datatableInstance);
        const selectedRows = this.getSelectedRows();
        if (selectedRows.length) this.dispatchEvent(new CustomEvent("rowselection", { detail: { selectedRows, config: {} } }));
      }
      if (isFieldSelector(selector) && this.variant === undefined) this.variant = "standard";
      this.reportUnsupportedAttributes();
      if (selector === "lightning-input-field") registerFormFieldWithNearestForm(this, "__gladeInputFields");
      if (selector === "lightning-output-field") registerFormFieldWithNearestForm(this, "__gladeOutputFields");
      if (selector === "lightning-messages") registerFormFieldWithNearestForm(this, "__gladeMessages");
      if (isRecordFormSelector(selector)) {
        this.__formButtonClick ||= this.handleFormButtonClick.bind(this);
        this.addEventListener("click", this.__formButtonClick);
      }
    }

    renderedCallback() {
      // Slotted fields have connected by this point; their field names belong
      // in the record request even when the form has no fields property.
      if (isRecordFormSelector(selector)) this.loadRecord();
      if (isFieldSelector(selector)) hydrateReferenceField(this);
      if (selector === "lightning-input-field") hydratePersonNameField(this);
    }

    disconnectedCallback() {
      if (this.__formButtonClick) this.removeEventListener("click", this.__formButtonClick);
      for (const adapter of this.__referenceAdapters || []) adapter.disconnect();
      this.__referenceKey = undefined;
      this.__personNameAdapter?.disconnect();
      this.__personNameKey = undefined;
      const form = this.__gladeForm;
      if (form) {
        for (const key of ["__gladeInputFields", "__gladeOutputFields", "__gladeMessages"]) {
          if (form[key]) form[key] = form[key].filter(field => field !== this);
        }
      }
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
      if (isFieldSelector(selector)) {
        this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value: this.value } }));
        return;
      }
      const detail = { value: this.value, checked: Boolean(target.checked) };
      const fieldName = normalizeFieldName(this.fieldName || this.name);
      if (fieldName) detail.fieldName = fieldName;
      this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail }));
    }

    toggleFieldPicklist(event) {
      event.stopPropagation();
      const member = event.currentTarget.dataset.namePart || "";
      this.__openPicklist = this.__openPicklist === member ? undefined : member;
    }

    selectFieldPicklist(event) {
      event.stopPropagation();
      const { value, namePart } = event.currentTarget.dataset;
      this.__openPicklist = undefined;
      if (namePart) {
        this.value = { ...this.value, [namePart]: value };
        const name = this.value;
        this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: {
          salutation: name.Salutation, firstName: name.FirstName, middleName: name.MiddleName,
          lastName: name.LastName, informalName: "", suffix: name.Suffix, validity: {},
        } }));
      } else {
        this.value = value;
        this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value } }));
      }
    }

    handleDateChange(event) {
      event.stopPropagation();
      const input = event.target;
      const text = input.value;
      const parsed = parseFieldDateText(text);
      const retainInvalid = text !== "" && !parsed && this.value == null;
      this.__dateText = retainInvalid ? text : undefined;
      this.__fieldError = retainInvalid
        ? `${this.label || normalizeFieldName(this.fieldName)}\nYour entry does not match the allowed format ${dateFormatExample()}.` : "";
      this.value = parsed;
      input.value = retainInvalid ? text : displayFieldDate(parsed);
      this.dispatchEvent(new CustomEvent("change", { bubbles: true, composed: true, detail: { value: parsed } }));
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
      // Native submit(fields) is the direct mutation route. The cancelable
      // submit event belongs to the user pressing a submit button.
      if (isRecordFormSelector(selector)) {
        this.saveRecord(submitFields);
        return;
      }
      const submitEvent = new CustomEvent("submit", {
        bubbles: true, composed: true, cancelable: true, detail: { fields: submitFields },
      });
      this.dispatchEvent(submitEvent);
      if (!submitEvent.defaultPrevented) this.saveRecord(submitFields);
    }

    handleFormButtonClick(event) {
      if (event.defaultPrevented || selector === "lightning-record-view-form") return;
      const button = event.composedPath?.().find(node => node?.tagName === "LIGHTNING-BUTTON" && node.type === "submit");
      if (button) this.handleSubmit(event);
    }

    handleInlineEdit() {
      this.__editing = true;
      this.emitRecordLoad();
    }

    handleOutputEdit() {
      this.dispatchEvent(new CustomEvent("edit"));
    }

    handleCancel(event) {
      event?.preventDefault?.();
      const fields = this.collectRecordFormFields();
      for (const field of inputFieldsForForm(this)) field.reset?.();
      if (selector === "lightning-record-form") {
        this.__editing = false;
        this.dispatchEvent(new CustomEvent("cancel"));
        this.emitRecordLoad();
      } else {
        this.dispatchEvent(new CustomEvent("cancel", { bubbles: true, composed: true, detail: { fields } }));
      }
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
      if (!this.recordId && ["lightning-record-edit-form", "lightning-record-form"].includes(selector)) {
        this.loadCreateDefaults();
        return;
      }
      if (!this.recordId) return;
      this.__recordLoaded = true;
      const fields = this.fields === undefined ? fieldComponentsForForm(this).map(field => field.fieldName) : this.fields;
      fetch("/lightning/wire/getRecordUi", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify({
          recordIds: [this.recordId],
          fields: recordFieldRefs(this.objectApiName, fields),
        }),
      }).then((response) => response.json()).then((result) => {
        if (result?.error) {
          const body = result.error.body || result.error;
          const detail = { message: body.message, output: body.output || {} };
          this.error = detail;
          applyRecordFormErrors(this, detail);
          this.dispatchEvent(new CustomEvent("error", { detail }));
          return;
        }
        const data = result?.data || result;
        this.value = data?.records?.[this.recordId];
        this.__recordLoadDetail = {
          records: data?.records,
          objectInfos: data?.objectInfos,
          layouts: data?.layouts,
          layoutUserStates: data?.layoutUserStates,
          picklistValues: data?.picklistValues,
        };
        this.data = { record: this.value, objectInfos: data?.objectInfos || {}, objectInfo: data?.objectInfos?.[this.objectApiName], layouts: data?.layouts };
        applyRecordUiToFormFields(this, this.data);
        this.emitRecordHydrationLoads();
      }).catch((err) => {
        const detail = { message: err?.message || String(err) };
        this.error = detail;
        this.dispatchEvent(new CustomEvent("error", { detail }));
      });
    }

    emitRecordLoad() {
      this.dispatchEvent(new CustomEvent("load", { detail: this.__recordLoadDetail }));
    }

    emitRecordHydrationLoads(createDefaults = false, cachedDefaults = false) {
      this.emitRecordLoad();
      if (selector === "lightning-record-form") {
        this.emitRecordLoad();
      } else if (createDefaults && cachedDefaults &&
        inputFieldsForForm(this).length === 1 &&
        inputFieldsForForm(this).some(field => !field.__explicitFieldValue)) {
        this.emitRecordLoad();
      } else if (fieldComponentsForForm(this).some(field =>
        field.__fieldMetadata?.dataType === "Boolean" && field.__fieldMetadata.updateable === false)) {
        this.emitRecordLoad();
      }
    }

    loadCreateDefaults() {
      if (this.__recordLoaded) return;
      this.__recordLoaded = true;
      const request = {
        objectApiName: this.objectApiName,
        fields: recordFieldRefs(this.objectApiName, this.fields),
      };
      // Defaults transport can be shared while distinct slotted field
      // compositions have their own hydration sequence.
      const cacheKey = JSON.stringify([request, fieldComponentsForForm(this).map(field => normalizeFieldName(field.fieldName))]);
      const cachedDefaults = hydratedCreateDefaults.has(cacheKey);
      fetch("/lightning/wire/getRecordCreateDefaults", {
        method: "POST",
        headers: { "Content-Type": "application/json" },
        body: JSON.stringify(request),
      }).then((response) => response.json()).then((result) => {
        if (result?.error) {
          this.error = result.error;
          this.dispatchEvent(new CustomEvent("error", { detail: result.error }));
          return;
        }
        const data = result?.data || result;
        this.value = data?.record || data;
        this.__recordLoadDetail = {
          layout: data?.layout,
          objectInfos: data?.objectInfos,
          record: data?.record,
          picklistValues: data?.picklistValues,
        };
        this.data = { record: this.value, objectInfos: data?.objectInfos || {}, objectInfo: data?.objectInfos?.[this.objectApiName], layout: data?.layout };
        applyCreateDefaultsToFields(this, data);
        hydratedCreateDefaults.add(cacheKey);
        this.emitRecordHydrationLoads(true, cachedDefaults);
      }).catch((err) => {
        const detail = { message: err?.message || String(err) };
        this.error = detail;
        this.dispatchEvent(new CustomEvent("error", { detail }));
      });
    }

    saveRecord(fields) {
      if (isRecordFormSelector(selector)) {
        let mutation;
        try {
          const record = this.data.record;
          mutation = this.recordId
            ? updateRecord({ fields: { ...fields, Id: record.id } })
            : createRecord({ apiName: record.apiName, fields });
        } catch (error) {
          const detail = { message: error?.message || String(error), output: {} };
          this.error = detail;
          applyRecordFormErrors(this, detail);
          this.dispatchEvent(new CustomEvent("error", { detail }));
          return;
        }
        mutation.then(record => {
          if (this.recordId) {
            this.value = record;
            if (this.__recordLoadDetail?.records) {
              this.__recordLoadDetail = {
                ...this.__recordLoadDetail,
                records: { ...this.__recordLoadDetail.records, [record.id]: record },
              };
            }
            if (this.data) {
              this.data = { ...this.data, record };
              applyRecordUiToFormFields(this, this.data);
            }
          }
          this.dispatchEvent(new CustomEvent("success", {
            bubbles: true, composed: true, detail: record,
          }));
          if (this.recordId) this.emitRecordLoad();
        }).catch(error => {
          const body = error?.body || error;
          const detail = {
            message: body?.message,
            detail: body?.detail,
            output: body?.output === undefined ? {} : body.output,
          };
          this.error = detail;
          applyRecordFormErrors(this, detail);
          this.dispatchEvent(new CustomEvent("error", { detail }));
        });
        return;
      }
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
      if (isFieldSelector(selector)) {
        this.__fieldError = Array.isArray(errors)
          ? errors.map(error => [error.fieldLabel || this.label, error.message].filter(Boolean).join("\n")).join("\n")
          : errors?.message || "";
      }
    }

    getWiredData() {
      return this.__wiredData;
    }

    wireRecordUi(data) {
      this.__wiredData = data;
      applyRecordUiToField(this, data);
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
      if (isFieldSelector(selector)) {
        this.__fieldValue = this.__initialValue;
        this.__fieldError = "";
        this.__dateText = undefined;
        this.__openPicklist = undefined;
      } else this.value = this.__initialValue ?? "";
      this.dirty = false;
      this.__errors = null;
      this.__customValidityMessage = "";
      const control = formControl(this);
      if (this.__personNameMembers?.length) syncControlValue(control, this);
      else if (control && "value" in control) control.value = this.value ?? "";
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
      const message = validityMessage({ required: validationRequired(this), value: this.value, customError: this.__customValidityMessage, type: inputTypeForField(this) });
      control?.setCustomValidity?.(message || "");
      if (message) return false;
      return control?.checkValidity ? control.checkValidity() : true;
    }

    reportValidity() {
      const control = formControl(this);
      syncControlValue(control, this);
      const message = validityMessage({ required: validationRequired(this), value: this.value, customError: this.__customValidityMessage, type: inputTypeForField(this) });
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
      if (selector === "lightning-datatable") {
        const selected = new Set(datatableSelectedKeys(this).map(String));
        return datatableEntries(this).filter((entry) => selected.has(String(entry.name))).map((entry) => entry.row);
      }
      const keyField = this.keyField || "id";
      const selected = new Set((this.selectedRows || []).map(String));
      return (this.data || []).filter((row) => selected.has(String(row?.[keyField])));
    }

    handleRowSelection(event) {
      event.stopPropagation();
      const entry = datatableEntries(this)[Number(event.currentTarget.dataset.rowIndex)];
      if (!entry || event.currentTarget.disabled) return;
      const checked = event.currentTarget.checked;
      const selected = new Set(datatableSelectedKeys(this).map(String));
      if (checked) {
        if (datatableSelectionLimit(this) === 1) selected.clear();
        selected.add(String(entry.name));
      } else selected.delete(String(entry.name));
      this.__datatableSelectedKeys = datatableEntries(this).filter((row) => selected.has(String(row.name))).map((row) => row.name);
      this.dispatchEvent(new CustomEvent("rowselection", {
        detail: { selectedRows: this.getSelectedRows(), config: { action: checked ? "rowSelect" : "rowDeselect", value: entry.name } },
      }));
    }

    handleSort(event) {
      event.preventDefault();
      event.stopPropagation();
      const fieldName = event?.currentTarget?.dataset?.fieldName || "";
      if (!fieldName) return;
      const sortDirection = this.sortedBy === fieldName && this.sortedDirection === "asc" ? "desc" : "asc";
      this.sortedBy = fieldName;
      this.sortedDirection = sortDirection;
      this.dispatchEvent(new CustomEvent("sort", {
        detail: { fieldName, sortDirection, fieldNames: [fieldName], sortDirections: [sortDirection], isMultiColumnSort: false },
      }));
    }

    openInlineEdit() {
      const columnIndex = (this.columns || []).findIndex((column) => column.editable);
      if (columnIndex >= 0 && datatableEntries(this).length) this.__datatableEditCell = { rowIndex: 0, columnIndex };
    }

    handleDatatableEdit(event) {
      event.stopPropagation();
      const dataset = event.currentTarget.dataset;
      this.__datatableEditCell = { rowIndex: Number(dataset.rowIndex), columnIndex: Number(dataset.columnIndex) };
    }

    handleCellChange(event) {
      if (event.type === "keydown" && event.key !== "Enter") return;
      if (!this.__datatableEditCell) return;
      event.stopPropagation();
      const dataset = event?.currentTarget?.dataset || {};
      const entry = datatableEntries(this)[Number(dataset.rowIndex)];
      const fieldName = dataset.fieldName || "";
      if (!entry || !fieldName) return;
      const value = event?.currentTarget?.type === "checkbox" ? Boolean(event.currentTarget.checked) : event?.currentTarget?.value;
      const column = (this.columns || [])[Number(dataset.columnIndex)];
      this.__datatableEditCell = undefined;
      if (["number", "currency", "percent"].includes(column?.type) && value !== "" && !Number.isFinite(Number(value))) return;
      const keyField = this.keyField || "id";
      const drafts = datatableDraftValues(this).map((draft) => ({ ...draft }));
      const existing = drafts.find((draft) => String(draft[keyField]) === String(entry.name));
      if (existing) existing[fieldName] = value;
      else drafts.push({ [keyField]: entry.name, [fieldName]: value });
      this.__datatableDraftValues = drafts;
      this.dispatchEvent(new CustomEvent("cellchange", { detail: { draftValues: drafts } }));
    }

    handleDatatableSave() {
      const draftValues = datatableDraftValues(this).slice();
      this.dispatchEvent(new CustomEvent("save", { detail: { draftValues } }));
    }

    handleDatatableCancel() {
      this.__datatableDraftValues = undefined;
      this.__datatableEditCell = undefined;
      this.draftValues = [];
      this.dispatchEvent(new CustomEvent("cancel", { bubbles: true, composed: true, detail: { draftValues: [] } }));
    }

    handleLoadMore() {
      this.dispatchEvent(new CustomEvent("loadmore", { bubbles: true, composed: true, detail: {} }));
    }

    handleDatatableActionMenu(event) {
      event.stopPropagation();
      const dataset = event.currentTarget.dataset;
      const menu = dataset.rowIndex + ":" + dataset.columnIndex;
      this.__datatableActionMenu = this.__datatableActionMenu === menu ? undefined : menu;
    }

    handleRowAction(event) {
      event.preventDefault();
      event.stopPropagation();
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
      if (!row || !action || action.disabled) return;
      this.__datatableActionMenu = undefined;
      this.dispatchEvent(new CustomEvent("rowaction", { detail: { action, row } }));
    }
  }

  const decorators = {
    publicProps: publicProps(selector),
    publicMethods: COMMON_METHODS,
  };
  if (selector === "lightning-datatable") {
    // User interactions keep private state until the application assigns the
    // public input again, including an assignment of the same array reference.
    Object.defineProperties(LocalBaseComponent.prototype, {
      selectedRows: {
        configurable: true,
        enumerable: true,
        get() { return this.__datatableSelectedRowsInput; },
        set(value) {
          this.__datatableSelectedRowsInput = value;
          this.__datatableSelectedKeys = undefined;
        },
      },
      draftValues: {
        configurable: true,
        enumerable: true,
        get() { return this.__datatableDraftValuesInput; },
        set(value) {
          this.__datatableDraftValuesInput = value;
          this.__datatableDraftValues = undefined;
        },
      },
    });
    decorators.publicMethods = [...COMMON_METHODS, "openInlineEdit"];
    decorators.fields = ["__datatableSelectedKeys", "__datatableActionMenu", "__datatableEditCell", "__datatableDraftValues",
      "__datatableSelectedRowsInput", "__datatableDraftValuesInput"];
  }
  if (isFieldSelector(selector)) {
    Object.defineProperties(LocalBaseComponent.prototype, {
      value: {
        configurable: true,
        get() { return this.__fieldValue; },
        set(value) { this.__explicitFieldValue = value !== undefined; this.__fieldValue = value; },
      },
      required: {
        configurable: true,
        get() { return Boolean(this.__required || this.__fieldMetadata?.required || this.__personNameMembers?.some(member => member.metadata.required) || immutableBooleanField(this)); },
        set(value) { this.__required = Boolean(value); },
      },
    });
  }
  const track = isFieldSelector(selector) ? { __fieldValue: 1, __fieldMetadata: 1, __fieldError: 1, __required: 1, __openPicklist: 1, __dateText: 1,
    __referenceRecord: 1, __referenceInfo: 1, __personNameMembers: 1 } :
    isRecordFormSelector(selector) ? { __editing: 1 } : {};
  if (isFieldSelector(selector) || isRecordFormSelector(selector)) decorators.track = track;
  registerDecorators(LocalBaseComponent, decorators);
  render.stylesheets = [];
  render.slots = [""];
  const template = registerTemplate(render);
  freezeTemplate(render);
  return registerComponent(LocalBaseComponent, { tmpl: template, sel: selector, apiVersion: 63 });
}

function renderInputField($api, $cmp) {
  const { h, t, b } = $api;
  if (!$cmp.__fieldMetadata) return [];
  if ($cmp.__personNameMembers?.length) return renderPersonNameField($api, $cmp);
  const reference = referenceFieldInfo($cmp);
  if (reference) return renderReferenceInputField($api, $cmp, reference);
  const metadataType = String($cmp.__fieldMetadata.dataType || $cmp.__fieldMetadata.type || "").toLowerCase();
  const type = inputTypeForField($cmp) === "checkbox" ? "checkbox" : "text";
  const label = (type !== "checkbox" && $cmp.required ? "*" : "") + ($cmp.label || normalizeFieldName($cmp.fieldName) || "");
  const labelNode = h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t(label)]);
  const errorNode = h("div", { classMap: { "slds-form-element__help": true }, attrs: { style: "white-space: pre-line" }, key: 3 }, [t($cmp.__fieldError || "")]);
  const options = $cmp.options || picklistValuesForField($cmp, $cmp.fieldName);
  if (metadataType === "picklist" || options.length) {
    return [h("div", { classMap: { "slds-form-element": true }, key: 0 }, $api.f([labelNode,
      ...renderFieldPicklist($api, $cmp, options, $cmp.value, "", 2), errorNode,
    ]))];
  }
  const multiline = metadataType === "textarea" || $cmp.__fieldMetadata.extraTypeInfo === "PlainTextArea";
  const date = metadataType === "date";
  const displayValue = date ? $cmp.__dateText ?? displayFieldDate($cmp.value) : $cmp.value;
  const controls = [labelNode, h(multiline ? "textarea" : "input", {
    classMap: { "slds-input": !multiline, "slds-textarea": multiline },
    attrs: { type: multiline ? undefined : type, "data-field-name": normalizeFieldName($cmp.fieldName) || undefined,
      "aria-invalid": $cmp.__fieldError ? "true" : !date && type !== "checkbox" && $cmp.value !== false && $cmp.value !== null && $cmp.value !== undefined && $cmp.value !== "" ? "false" : undefined },
    props: {
      value: type === "checkbox" ? "" : (displayValue === false ? "" : displayValue ?? ""),
      checked: type === "checkbox" ? Boolean($cmp.value || $cmp.checked) : undefined,
      disabled: Boolean($cmp.disabled), required: type === "checkbox" ? false : Boolean($cmp.required), readOnly: Boolean($cmp.readOnly),
    }, key: 2, on: { change: b(date ? $cmp.handleDateChange : $cmp.handleChange) },
  })];
  if (date) {
    const title = "Select a date for " + ($cmp.label || normalizeFieldName($cmp.fieldName));
    controls.push(h("button", { attrs: { type: "button", title, style: "display:block" }, key: 4 }, [t(title)]));
    if (!$cmp.__fieldError) controls.push(h("div", { classMap: { "slds-form-element__help": true }, key: 5 }, [t("Format: " + dateFormatExample())]));
  }
  if (!multiline || $cmp.__fieldError) controls.push(errorNode);
  return [h("div", { classMap: { "slds-form-element": true }, key: 0 }, $api.f(controls))];
}

function referenceFieldInfo(component) {
  const metadata = component.__fieldMetadata;
  const targets = metadata?.referenceToInfos;
  if (metadata?.dataType !== "Reference" || !Array.isArray(targets) || targets.length !== 1 ||
    !targets[0].apiName || !Array.isArray(targets[0].nameFields) || !targets[0].nameFields.length) return null;
  const value = component.value;
  if (value != null && value !== "" && (typeof value !== "string" || !/^(?:[a-zA-Z0-9]{15}|[a-zA-Z0-9]{18})$/.test(value))) return null;
  return targets[0];
}

function personNameMembers(field, data) {
  if (!data?.record || field.__fieldMetadata?.dataType !== "String") return [];
  if (field.__explicitFieldValue && field.value !== null && typeof field.value !== "object") return [];
  const fields = data.objectInfo?.fields || data.objectInfos?.[data.record.apiName]?.fields || {};
  const parent = normalizeFieldName(field.fieldName);
  // Captured standard presentation order; every role must belong to this
  // parent in the schema. Existing unrelated fields never establish a group.
  const roles = ["Salutation", "FirstName", "LastName"];
  const members = roles.map(name => ({ name, metadata: fields[name] })).filter(member => member.metadata?.compoundFieldName === parent);
  return members.length === roles.length ? members : [];
}

function hydratePersonNameField(field) {
  if (!field.__personNameMembers?.length || field.__explicitFieldValue) return;
  const data = field.getWiredData?.();
  const recordId = data?.record?.id;
  const apiName = data?.record?.apiName;
  if (!recordId || !apiName) return;
  const members = field.__personNameMembers;
  const key = JSON.stringify([recordId, apiName, members.map(member => member.name)]);
  if (key === field.__personNameKey) return;
  field.__personNameKey = key;
  field.__personNameAdapter?.disconnect();
  const adapter = new getRecord(({ data: record }) => {
    if (!record || field.__personNameKey !== key || field.__explicitFieldValue) return;
    const value = Object.fromEntries(members.map(({ name }) => {
      const entry = record.fields?.[name];
      return [name, entry && typeof entry === "object" ? entry.value : entry];
    }));
    field.__fieldValue = value;
    field.__initialValue = value;
  });
  field.__personNameAdapter = adapter;
  adapter.connect();
  adapter.update({ recordId, fields: members.map(member => `${apiName}.${member.name}`) });
}

function renderPersonNameField($api, $cmp) {
  const { h, t } = $api;
  const value = $cmp.value;
  // The captured null compound value raises at FirstName, before any
  // controls mount. Do not coerce it into an empty record or text input.
  const firstName = value.FirstName;
  const lastName = value.LastName;
  const salutation = value.Salutation;
  const values = { FirstName: firstName, LastName: lastName, Salutation: salutation };
  const children = [h("div", { classMap: { "slds-form-element__label": true }, attrs: { style: "display:block" }, key: 1 }, [t(($cmp.required ? "*" : "") + "Name")])];
  for (const [index, member] of $cmp.__personNameMembers.entries()) {
    const metadata = member.metadata;
    const partValue = values[member.name];
    children.push(h("div", { classMap: { "slds-form-element__label": true }, key: 10 + index },
      [t((metadata.required ? "*" : "") + metadata.label)]));
    if (metadata.dataType === "Picklist") {
      children.push(...renderFieldPicklist($api, $cmp, metadata.picklistValues || [], partValue, member.name, 20 + index));
    } else {
      children.push(h("input", { classMap: { "slds-input": true }, attrs: { type: "text", placeholder: metadata.label,
        "aria-invalid": partValue ? "false" : null },
        props: { value: partValue ?? "", disabled: Boolean($cmp.disabled), required: Boolean(metadata.required), readOnly: Boolean($cmp.readOnly) },
        key: 20 + index }));
    }
    children.push(h("div", { classMap: { "slds-form-element__help": true }, key: 30 + index }, []));
  }
  return [h("div", { classMap: { "slds-form-element": true }, key: 0 }, $api.f(children))];
}

function hydrateReferenceField(component) {
  const reference = referenceFieldInfo(component);
  const key = reference ? JSON.stringify([reference.apiName, reference.nameFields, component.value]) : "";
  if (key === component.__referenceKey) return;
  component.__referenceKey = key;
  for (const adapter of component.__referenceAdapters || []) adapter.disconnect();
  component.__referenceAdapters = [];
  component.__referenceRecord = undefined;
  component.__referenceInfo = undefined;
  if (!reference) return;
  const info = new getObjectInfo(({ data }) => {
    if (data && component.__referenceKey === key) component.__referenceInfo = data;
  });
  component.__referenceAdapters.push(info);
  info.connect();
  info.update({ objectApiName: reference.apiName });
  if (component.value) {
    const record = new getRecord(({ data }) => {
      if (data && component.__referenceKey === key) component.__referenceRecord = data;
    });
    component.__referenceAdapters.push(record);
    record.connect();
    record.update({ recordId: component.value, fields: reference.nameFields.map(name => `${reference.apiName}.${normalizeFieldName(name)}`) });
  }
}

function referenceDisplayValue(component, reference) {
  return reference.nameFields.map(name => readRecordDisplayValue(component.__referenceRecord, name)).filter(value => value != null && value !== "").join(" ");
}

function renderReferenceInputField($api, $cmp, reference) {
  const { h, t, b } = $api;
  const info = $cmp.__referenceInfo;
  const nameField = info?.fields?.[normalizeFieldName(reference.nameFields[0])];
  const label = nameField?.label || $cmp.label || "";
  const selected = Boolean($cmp.__referenceRecord);
  const clearLabel = "Clear " + label + " Selection";
  const children = [
    h("span", { classMap: { "slds-form-element__label": true }, key: 1 }, [t(($cmp.required ? "*" : "") + label)]),
    h("input", { classMap: { "slds-input": true }, attrs: { type: "text", placeholder: info?.labelPlural ? "Search " + info.labelPlural + "..." : "",
      "aria-invalid": $cmp.__fieldError ? "true" : null }, props: { value: referenceDisplayValue($cmp, reference),
      disabled: Boolean($cmp.disabled), required: Boolean($cmp.required), readOnly: Boolean(selected || $cmp.readOnly) },
      key: 2, on: { change: b($cmp.handleChange) } }),
  ];
  if (selected) {
    children.push(h("div", { key: 5 }, [t(info?.label || "")]));
    children.push(h("button", { attrs: { type: "button", title: clearLabel, style: "display:block" },
      props: { disabled: Boolean($cmp.disabled) }, key: 4 }, [t(clearLabel)]));
  }
  children.push(h("div", { classMap: { "slds-form-element__help": true }, attrs: { style: "white-space: pre-line" }, key: 3 }, [t($cmp.__fieldError || "")]));
  return [h("div", { classMap: { "slds-form-element": true }, key: 0 }, $api.f(children))];
}

function fieldDate(value) {
  if (typeof value !== "string" || !/^\d{4}-\d{2}-\d{2}$/.test(value)) return null;
  const date = new Date(value + "T00:00:00Z");
  return Number.isNaN(date.getTime()) ? null : date;
}

function renderFieldPicklist($api, component, suppliedOptions, value, member, key) {
  const { h, t, b } = $api;
  const options = [{ label: "--None--", value: "" }, ...suppliedOptions.filter(option => option.value !== "")];
  const selected = options.find(option => option.value === (value ?? ""));
  const nodes = [h("button", { attrs: { type: "button", style: "display:block", "data-name-part": member },
    props: { disabled: Boolean(component.disabled) }, key,
    on: { click: b(component.toggleFieldPicklist) } }, [t(selected?.label || value || "--None--")])];
  if (component.__openPicklist === member) {
    nodes.push(h("div", { attrs: { role: "listbox" }, key: 100 + key }, options.map((option, index) =>
      h("lightning-base-combobox-item", { attrs: { role: "option", "aria-selected": String(option.value === (value ?? "")),
        "data-value": option.value, "data-name-part": member, style: "display:block" }, key: index,
        on: { click: b(component.selectFieldPicklist) } }, [t(option.label)]))));
  }
  return nodes;
}

function dateFormatter() {
  return new Intl.DateTimeFormat(locale, { timeZone: "UTC", year: "numeric", month: "short", day: "numeric" });
}

function displayFieldDate(value) {
  const date = fieldDate(value);
  return date ? dateFormatter().format(date) : value ?? "";
}

function dateFormatExample() {
  return dateFormatter().format(new Date("2024-12-31T00:00:00Z"));
}

function parseFieldDateText(text) {
  // Parse the displayed locale format, then round-trip to reject calendar
  // overflow and unrelated text. Never store localized display text in LDS.
  const formatter = dateFormatter();
  const parts = formatter.formatToParts(new Date("2024-12-31T00:00:00Z"));
  const escape = value => value.replace(/[.*+?^${}()|[\]\\]/g, "\\$&");
  const pattern = parts.map(part => ["day", "year"].includes(part.type) ? "(\\d+)" :
    part.type === "month" ? "(.+?)" : escape(part.value)).join("");
  const match = new RegExp("^" + pattern + "$").exec(text);
  if (!match) return null;
  const values = {};
  let index = 1;
  for (const part of parts) if (["day", "month", "year"].includes(part.type)) values[part.type] = match[index++];
  const month = Array.from({ length: 12 }, (_, index) => formatter.formatToParts(new Date(Date.UTC(2024, index, 1)))
    .find(part => part.type === "month").value).indexOf(values.month);
  if (month < 0) return null;
  const date = new Date(Date.UTC(Number(values.year), month, Number(values.day)));
  return formatter.format(date) === text ? date.toISOString().slice(0, 10) : null;
}

function formatOutputFieldValue(value, metadata) {
  if (value === false) return "False";
  if (value === true) return "True";
  if (value === null || value === undefined || value === "") return "";
  if (metadata?.dataType === "Currency") return new Intl.NumberFormat(locale, {
    style: "currency", currency, minimumFractionDigits: 0, maximumFractionDigits: 0,
  }).format(value);
  const date = metadata?.dataType === "Date" ? fieldDate(value) : null;
  return date ? new Intl.DateTimeFormat(locale, { timeZone: "UTC", year: "numeric", month: "numeric", day: "numeric" }).format(date) : value;
}

function renderOutputField($api, $cmp) {
  const { h, t, b } = $api;
  if (!$cmp.__fieldMetadata) return [];
  const value = $cmp.value ?? readRecordDisplayValue($cmp.record || $cmp.value, $cmp.fieldName || $cmp.name);
  const label = $cmp.label || normalizeFieldName($cmp.fieldName || $cmp.name);
  const reference = referenceFieldInfo($cmp);
  const display = reference && $cmp.__referenceRecord ? referenceDisplayValue($cmp, reference) : formatOutputFieldValue(value, $cmp.__fieldMetadata);
  const editLabel = "Edit: " + normalizeFieldName($cmp.fieldName);
  const text = label + (display !== "" && display !== null && display !== undefined ? "\n" + display : "") + ($cmp.editable ? "\n" : "");
  return [h("div", { classMap: { "slds-form-element": true }, key: 0 }, [
    h("div", { classMap: { "slds-form-element__static": true }, attrs: { style: "white-space: pre-line" }, key: 1 }, [t(text)]),
    $cmp.editable ? h("button", { attrs: { type: "button", title: editLabel }, key: 2,
      on: { click: b($cmp.handleOutputEdit) } }, [t(editLabel)]) : null,
  ].filter(Boolean))];
}

function renderMessages($api, $cmp, $slotset) {
  const error = $cmp.error?.body || $cmp.error;
  return [$api.h("div", { classMap: { "slds-notify": true, "slds-notify_alert": true }, attrs: { role: error ? "alert" : "status" }, key: 0 }, [
    error ? $api.t(error.message || "") : $api.s("", { key: 1 }, [], $slotset),
  ])];
}

function renderRecordForm($api, $cmp, $slotset) {
  const { h, t, s, b, c } = $api;
  const autoFields = $cmp.__selector === "lightning-record-form";
  const editMode = autoFields && ($cmp.__editing ?? ($cmp.mode === "edit"));
  const children = [];
  const messages = ($cmp.hostElement || $cmp.template?.host)?.__gladeMessages || [];
  if ($cmp.error && !messages.length) children.push(h("div", { attrs: { role: "alert" }, key: 6 }, [t(($cmp.error.body || $cmp.error).message || "")]));
  if (autoFields) children.push(h("div", { key: 2 }, fieldList($cmp.fields).map((field, index) => {
    const name = normalizeFieldName(field);
    const entry = $cmp.value?.fields?.[name];
    const value = entry && typeof entry === "object" ? entry.value : entry;
    return c(editMode ? "lightning-input-field" : "lightning-output-field", editMode ? createInputField() : createOutputField(), {
      key: (editMode ? 40 : 20) + index,
      props: { fieldName: name, value, editable: !editMode && $cmp.mode !== "readonly" },
      on: { edit: b($cmp.handleInlineEdit) },
    });
  })));
  if (editMode) {
    children.push(h("button", { classMap: { "slds-button": true, "slds-button_neutral": true }, attrs: { type: "button" }, key: 5, on: { click: b($cmp.handleCancel) } }, [t("Cancel")]));
    children.push(h("button", { classMap: { "slds-button": true, "slds-button_brand": true }, attrs: { type: "submit" }, key: 3 }, [t("Save")]));
  }
  children.push(s("", { key: 4 }, [], $slotset));
  return [h("form", {
    classMap: { "slds-form": true },
    attrs: { "data-object-api-name": $cmp.objectApiName || "" },
    key: 0,
    on: { submit: b($cmp.handleSubmit), change: b($cmp.handleFormFieldChange), click: b($cmp.handleFormButtonClick) },
  }, $api.f(children))];
}

function renderDatatable($api, $cmp) {
  const { h, t, b } = $api;
  const columns = Array.isArray($cmp.columns) ? $cmp.columns : [];
  const entries = datatableEntries($cmp);
  const selected = new Set(datatableSelectedKeys($cmp).map(String));
  const disabledRows = new Set(($cmp.disabledRows || []).map(String));
  const limit = datatableSelectionLimit($cmp);
  const hasEditable = columns.some((column) => column.editable);
  const numbered = $cmp.showRowNumberColumn || hasEditable;
  if (!columns.length) return [h("div", { classMap: { "slds-table_edit_container": true }, attrs: { style: "min-height:1px" }, key: 9 }, [])];
  const headers = [];
  if (numbered) headers.push(h("th", { key: "number-header" }, []));
  if (!$cmp.hideCheckboxColumn) {
    headers.push(h("th", { key: "selection-header" }, limit === 1 ? [] : [
      h("input", { attrs: { type: "checkbox", id: $cmp.__datatableID + "-all" },
        props: { checked: Boolean(selected.size), indeterminate: selected.size > 0 && selected.size < entries.length, disabled: !entries.length || limit === 0 }, key: "select-all" }),
      h("label", { classMap: { "slds-assistive-text": true }, attrs: { for: $cmp.__datatableID + "-all" }, key: "select-all-label" }, [t("Select All")]),
    ]));
  }
  for (const [columnIndex, column] of columns.entries()) {
    const label = column.label || column.fieldName || "";
    const children = [];
    if (column.type !== "action") {
      children.push(column.sortable
        ? h("a", { attrs: { href: "javascript:void(0);", "data-field-name": column.fieldName || "" }, key: "sort", on: { click: b($cmp.handleSort) } }, [t("Sort by: " + label)])
        : h("span", { key: "label" }, [t(label)]));
      if (column.type !== "boolean" && column.type !== "date-local") {
        const title = "Show " + label + " column actions";
        children.push(h("button", { attrs: { type: "button", title }, key: "column-actions" }, [t(title)]));
      }
      children.push(h("input", { attrs: { type: "range", min: "0", max: "1000", style: $cmp.__datatableEditCell ? "display:none" : undefined },
        props: { value: "1000" }, key: "resize" }));
    }
    headers.push(h("th", { key: "column-" + columnIndex }, children));
  }
  const tableChildren = [h("table", { classMap: { "slds-table": true, "slds-table_cell-buffer": true, "slds-table_bordered": true }, key: 0 }, [
    h("thead", { key: 1 }, [h("tr", { key: 2 }, $api.i(headers, (header) => header))]),
    h("tbody", { key: 3 }, $api.i(entries, (entry) => {
      const { row, rowIndex, name } = entry;
      const checked = selected.has(String(name));
      const cells = [];
      if (numbered) cells.push(h("td", { key: "number" }, []));
      if (!$cmp.hideCheckboxColumn) {
        const id = $cmp.__datatableID + "-row-" + rowIndex;
        cells.push(h("td", { key: "selection" }, [
          h("input", { attrs: { type: limit === 1 ? "radio" : "checkbox", id, name: $cmp.__datatableID, "data-row-index": String(rowIndex) },
            props: { checked, disabled: disabledRows.has(String(name)) || limit === 0 || (limit > 1 && !checked && selected.size >= limit) },
            key: "row-input", on: { change: b($cmp.handleRowSelection) } }),
          h("label", { classMap: { "slds-assistive-text": true }, attrs: { for: id }, key: "row-label" }, [t("Select Item " + (rowIndex + 1))]),
        ]));
      }
      for (const [columnIndex, column] of columns.entries()) {
        cells.push(h(columnIndex === 0 ? "th" : "td", { attrs: { scope: columnIndex === 0 ? "row" : undefined }, key: "cell-" + columnIndex },
          datatableCell($api, $cmp, row, rowIndex, column, columnIndex, name)));
      }
      return h("tr", { attrs: { "data-row-key": String(name), "aria-selected": $cmp.hideCheckboxColumn ? undefined : String(checked) },
        key: "row-" + String(name) }, $api.i(cells, (cell) => cell));
    })),
  ]),
  datatableDraftValues($cmp).length ? h("div", { classMap: { "slds-m-top_x-small": true }, key: 4 }, [
      h("button", { classMap: { "slds-button": true, "slds-button_brand": true }, attrs: { type: "button" }, key: 40, on: { click: b($cmp.handleDatatableSave) } }, [t("Save")]),
      h("button", { classMap: { "slds-button": true, "slds-button_neutral": true }, attrs: { type: "button" }, key: 41, on: { click: b($cmp.handleDatatableCancel) } }, [t("Cancel")]),
    ]) : null,
  $cmp.enableInfiniteLoading ? h("button", { classMap: { "slds-button": true, "slds-button_neutral": true }, attrs: { type: "button" }, key: 5,
    on: { click: b($cmp.handleLoadMore) } }, [t("Load More")]) : null];
  return [h("div", { classMap: { "slds-table_edit_container": true }, key: 9 }, tableChildren)];
}

function datatableCell($api, $cmp, row, rowIndex, column, colIndex, rowKey) {
  const { h, t, b } = $api;
  const type = String(column.type || "text").toLowerCase();
  if (type === "action") {
    const actions = column.typeAttributes?.rowActions || [];
    if (!actions.length) return [];
    return [h("button", { attrs: { type: "button", "data-row-index": String(rowIndex), "data-column-index": String(colIndex) }, key: "row-actions",
      on: { click: b($cmp.handleDatatableActionMenu) } }, [t("Show actions")]),
    $cmp.__datatableActionMenu === rowIndex + ":" + colIndex ? h("div", { attrs: { role: "menu" }, key: "menu" }, $api.i(actions, (action, actionIndex) =>
      h("a", { attrs: { href: "javascript:void(0)", role: "menuitem", "aria-disabled": action.disabled ? "true" : undefined,
        "data-row-index": String(rowIndex), "data-column-index": String(colIndex), "data-action-index": String(actionIndex) }, key: "action-" + actionIndex,
        on: { click: b($cmp.handleRowAction) } }, [t(action.label || action.name || "Action")])
    )) : null];
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
  const draft = datatableDraftValues($cmp).find((value) => String(value[$cmp.keyField || "id"]) === String(rowKey));
  const raw = draft && Object.prototype.hasOwnProperty.call(draft, column.fieldName) ? draft[column.fieldName] : row?.[column.fieldName];
  let content;
  if (type === "boolean") content = [h("span", { key: "boolean" }, [t(raw ? "True" : "False")])];
  else if (type === "url") content = raw ? [h("a", { attrs: { href: String(raw), target: column.typeAttributes?.target || undefined }, key: "url" }, [t(attrValue(column.typeAttributes?.label, row) || String(raw))])] : [];
  else if (type === "email") content = raw ? [h("a", { attrs: { href: `mailto:${raw}` }, key: "email" }, [t("Email " + raw)])] : [];
  else if (type === "phone") {
    const phone = String(raw ?? "");
    // dt_edit_phone_9 captures ten-digit display formatting at APIs 59 and 67;
    // dt_type_phone_0..3 retain international, short, null and empty values.
    const label = /^\d{10}$/.test(phone) ? `(${phone.slice(0, 3)}) ${phone.slice(3, 6)}-${phone.slice(6)}` : phone;
    content = phone ? [h("a", { attrs: { href: `tel:${phone}` }, key: "phone" }, [t(label)])] : [];
  } else if (type === "badge") content = [h("span", { classMap: { "slds-badge": true }, key: "badge" }, [t(raw ?? "")])];
  else content = [h("span", { key: "value" }, [t(formatDatatableValue(raw, type, column))])];
  if (column.editable) {
    content.push(t(" "));
    content.push(h("button", { attrs: { type: "button", "data-row-index": String(rowIndex), "data-column-index": String(colIndex) }, key: "edit",
      on: { click: b($cmp.handleDatatableEdit) } }, [t("Edit " + (column.label || column.fieldName || ""))]));
    const edit = $cmp.__datatableEditCell;
    content.push(edit?.rowIndex === rowIndex && edit.columnIndex === colIndex ? h("input", {
      classMap: { "slds-input": true }, attrs: { type: "text", "data-row-index": String(rowIndex), "data-column-index": String(colIndex), "data-field-name": column.fieldName || "" },
      props: { value: raw ?? "" }, key: "editor", on: { keydown: b($cmp.handleCellChange), blur: b($cmp.handleCellChange) },
    }) : null);
  }
  return content;
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
    ...(root?.template?.querySelectorAll?.("lightning-input-field") || []),
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
  // Queries expose DOM hosts, while connected fields register their component
  // instances. Count each physical field once and retain its private state.
  const unique = new Map();
  for (const field of fields) {
    const host = field.hostElement || field.template?.host || field;
    if (!unique.has(host) || field !== host) unique.set(host, field);
  }
  return [...unique.values()];
}

function fieldComponentsForForm(root) {
  const fields = [
    ...inputFieldsForForm(root),
    ...(root?.hostElement?.__gladeOutputFields || []),
    ...(root?.template?.host?.__gladeOutputFields || []),
  ];
  for (const selector of ["lightning-output-field"]) {
    fields.push(...(root?.template?.querySelectorAll?.(selector) || []));
    fields.push(...(root?.querySelectorAll?.(selector) || []));
    for (const container of [root, root?.hostElement, root?.template?.host]) {
      for (const element of container?.children || []) collectAssignedFields(element, fields, selector);
    }
    for (const slot of root?.template?.querySelectorAll?.("slot") || []) {
      for (const element of slot.assignedElements?.({ flatten: true }) || []) collectAssignedFields(element, fields, selector);
    }
  }
  return [...new Set(fields)];
}

function applyRecordFormErrors(form, error) {
  const detail = error?.body || error;
  const host = form?.hostElement || form?.template?.host;
  if (host) {
    host.__gladeRecordError = detail;
    for (const messages of host.__gladeMessages || []) messages.error = detail;
  }
  const fieldErrors = detail?.output?.fieldErrors || {};
  for (const field of inputFieldsForForm(form)) {
    const name = normalizeFieldName(field.fieldName);
    const errors = fieldErrors[name] || (field.required && (field.value === null || field.value === undefined || field.value === "")
      ? [{ fieldLabel: field.label, message: "Complete this field." }] : []);
    field.setErrors?.(errors);
  }
}

function applyRecordUiToFormFields(form, data) {
  const host = form?.hostElement || form?.template?.host;
  if (host) host.__gladeRecordUi = data;
  for (const field of fieldComponentsForForm(form)) field.wireRecordUi?.(data);
}

function layoutFieldLabel(data, fieldName) {
  // Layout item labels describe the control; component/object-info labels
  // describe the field itself and can differ for standard fields.
  function find(layout) {
    if (!layout || typeof layout !== "object") return undefined;
    for (const section of layout.sections || []) {
      for (const row of section.layoutRows || []) {
        for (const item of row.layoutItems || []) {
          if (item.layoutComponents?.some(component => component.componentType === "Field" && component.apiName === fieldName)) {
            return item.label;
          }
        }
      }
    }
    for (const value of Object.values(layout)) {
      if (value && typeof value === "object" && !Array.isArray(value)) {
        const label = find(value);
        if (label !== undefined) return label;
      }
    }
    return undefined;
  }
  return find(data?.layout || data?.layouts?.[data?.record?.apiName]);
}

function immutableBooleanField(field) {
  return field.__selector === "lightning-input-field" && field.__fieldMetadata?.dataType === "Boolean" && field.__fieldMetadata?.updateable === false;
}

function validationRequired(field) {
  // The immutable Boolean snapshot exposes required=true, but must not add a
  // new validation requirement beyond the caller and field metadata.
  return immutableBooleanField(field)
    ? Boolean(field.__required || field.__fieldMetadata?.required)
    : field.required;
}

function applyRecordUiToField(field, data) {
  const name = normalizeFieldName(field?.fieldName || field?.name);
  if (!name) return;
  const record = data?.record || {};
  const objectInfo = data?.objectInfo || data?.objectInfos?.[record?.apiName] || {};
  const recordField = record?.fields?.[name];
  const metadata = { ...(recordField && typeof recordField === "object" ? recordField : {}), ...(objectInfo?.fields?.[name] || {}) };
  field.__fieldMetadata = metadata;
  if (field.__selector === "lightning-input-field") field.__personNameMembers = personNameMembers(field, data);
  const value = recordField && typeof recordField === "object" ? recordField.value : recordField;
  if (value !== undefined) {
    if (isFieldSelector(field.__selector)) {
      if (!field.__explicitFieldValue) field.__fieldValue = value;
    } else field.value = value;
    field.__initialValue = value;
    field.dirty = false;
  }
  if (!field.label) field.label = (field.__selector === "lightning-input-field" ? layoutFieldLabel(data, name) : undefined) ?? metadata.label;
  if (field.__selector === "lightning-input-field" && metadata.dataType === "Boolean" && metadata.updateable === false) {
    field.disabled = true;
    field.readOnly = true;
  }
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

function collectAssignedInputFields(element, fields) {
  if (!element) return;
  if (element.tagName?.toLowerCase() === "lightning-input-field") fields.push(element);
  for (const child of element.querySelectorAll?.("lightning-input-field") || []) fields.push(child);
}

function collectAssignedFields(element, fields, selector) {
  if (!element) return;
  if (element.tagName?.toLowerCase() === selector) fields.push(element);
  for (const child of element.querySelectorAll?.(selector) || []) fields.push(child);
}

function registerFormFieldWithNearestForm(component, propertyName) {
  const host = component?.hostElement || component?.template?.host;
  let node = host?.parentElement || null;
  while (node) {
    const tag = node.tagName?.toLowerCase();
    if (tag === "lightning-record-edit-form" || tag === "lightning-record-form" || tag === "lightning-record-view-form") {
      node[propertyName] = node[propertyName] || [];
      if (!node[propertyName].includes(component)) node[propertyName].push(component);
      component.__gladeForm = node;
      if (node.__gladeRecordUi) component.wireRecordUi(node.__gladeRecordUi);
      if (propertyName === "__gladeMessages" && node.__gladeRecordError) component.error = node.__gladeRecordError;
      return;
    }
    const root = node.getRootNode?.();
    node = node.parentElement || root?.host || null;
  }
}

function applyCreateDefaultsToFields(form, data) {
  applyRecordUiToFormFields(form, { record: data?.record, objectInfos: data?.objectInfos, objectInfo: data?.objectInfos?.[form.objectApiName], layout: data?.layout });
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
      if (!field.__explicitFieldValue) field.__fieldValue = defaultValue;
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
    if (component.__selector === "lightning-datatable" && ["hide-checkbox-column", "max-row-selection", "sorted-by", "sorted-direction", "show-row-number-column"].includes(name)) continue;
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

function isFieldSelector(selector) {
  return selector === "lightning-input-field" || selector === "lightning-output-field";
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

function datatableEntries(component) {
  const keyField = component.keyField || "id";
  return (Array.isArray(component.data) ? component.data : []).map((row, rowIndex) => ({
    row, rowIndex, name: row?.[keyField] ?? "row-" + rowIndex,
  }));
}

function datatableSelectedKeys(component) {
  const selected = component.__datatableSelectedKeys ?? component.selectedRows;
  return Array.isArray(selected) ? selected : [];
}

function datatableSelectionLimit(component) {
  return component.maxRowSelection === undefined ? 1000 : Math.max(0, Math.floor(Number(component.maxRowSelection)));
}

function datatableDraftValues(component) {
  const drafts = component.__datatableDraftValues ?? component.draftValues;
  return Array.isArray(drafts) ? drafts : [];
}

function attrValue(value, row) {
  if (value && typeof value === "object" && value.fieldName) return row?.[value.fieldName];
  return value;
}

function formatDatatableValue(value, type, column) {
  if (type === "location") {
    const latitude = value.latitude;
    const longitude = value.longitude;
    return Number.isFinite(latitude) && Math.abs(latitude) <= 90 && Number.isFinite(longitude) && Math.abs(longitude) <= 180
      ? `${latitude}, ${longitude}` : "";
  }
  if (value === undefined || value === null) return "";
  const attrs = column.typeAttributes || {};
  if (["number", "currency", "percent"].includes(type)) {
    const options = {};
    for (const name of ["minimumIntegerDigits", "minimumFractionDigits", "maximumFractionDigits", "minimumSignificantDigits", "maximumSignificantDigits"]) {
      if (attrs[name] !== undefined) options[name] = attrs[name];
    }
    if (type === "currency") {
      options.style = "currency";
      options.currency = attrs.currencyCode || "USD";
    } else if (type === "percent") options.style = "percent";
    try {
      return new Intl.NumberFormat(undefined, options).format(Number(value));
    } catch {
      return String(value);
    }
  }
  if (type === "date" || type === "date-local") {
    const date = new Date(value);
    if (Number.isNaN(date.getTime())) return "";
    const options = type === "date-local" ? { timeZone: "UTC", year: "numeric", month: "short", day: "numeric" } : {};
    for (const name of ["timeZone", "year", "month", "day", "weekday", "hour", "minute", "second", "hour12"]) {
      if (attrs[name] !== undefined) options[name] = attrs[name];
    }
    if (!Object.keys(options).length) return date.toISOString().slice(0, 10);
    return new Intl.DateTimeFormat(undefined, options).format(date);
  }
  return String(value);
}
