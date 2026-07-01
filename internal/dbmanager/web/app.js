const apiBase = "/services/data/v65.0/glade/db-manager";
const root = document.getElementById("app");

const state = {
  objects: [],
  selectedObject: "",
  objectDetail: null,
  records: [],
  recordTotal: 0,
  query: "",
  objectQuery: "",
  includeDeleted: false,
  selectedRecordId: "",
  editing: null,
  error: "",
  fieldErrors: [],
  saving: false
};

const searchTimers = new Map();

function escapeHTML(value) {
  return String(value ?? "")
    .replaceAll("&", "&amp;")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;")
    .replaceAll('"', "&quot;")
    .replaceAll("'", "&#39;");
}

function attr(value) {
  return escapeHTML(value);
}

function normalizeName(value) {
  return String(value ?? "").toLowerCase();
}

function cssEscape(value) {
  if (window.CSS?.escape) {
    return CSS.escape(value);
  }
  return String(value).replace(/["\\]/g, "\\$&");
}

function focusSearch(selector, value) {
  const input = root.querySelector(selector);
  if (!input) {
    return;
  }
  input.focus();
  input.setSelectionRange(value.length, value.length);
}

function getFieldName(field) {
  return field.name || field.apiName || field.api_name || field.APIName || "";
}

function getFieldLabel(field) {
  return field.label || field.Label || getFieldName(field);
}

function getFieldControl(field) {
  return normalizeName(field.control || field.editor || field.type || "text");
}

function fieldIsCreateable(field) {
  return field.createable !== false && field.createable !== "false";
}

function fieldIsUpdateable(field) {
  return field.updateable !== false && field.updateable !== "false";
}

function fieldIsEditable(field) {
  if (!state.editing) {
    return false;
  }
  if (getFieldControl(field) === "readonly") {
    return false;
  }
  return state.editing.mode === "create" ? fieldIsCreateable(field) : fieldIsUpdateable(field);
}

function fieldValue(record, field) {
  const name = getFieldName(field);
  const fields = record?.fields || {};
  if (Object.prototype.hasOwnProperty.call(fields, name)) {
    return fields[name];
  }
  if (Object.prototype.hasOwnProperty.call(record || {}, name)) {
    return record[name];
  }
  return "";
}

function recordTitle(record) {
  return record?.title || record?.fields?.Name || record?.fields?.DeveloperName || record?.id || "";
}

function recordDeleted(record) {
  return Boolean(record?.deleted || record?.isDeleted || record?.system?.isDeleted);
}

function objectLabel(object) {
  return object?.label || object?.name || object?.apiName || "";
}

function objectName(object) {
  return object?.name || object?.apiName || "";
}

function picklistValues(field) {
  const values = field.picklistValues || field.picklist_values || field.values || [];
  return values.map((value) => {
    if (typeof value === "string") {
      return { value, label: value, active: true };
    }
    return {
      value: value.value ?? value.Value ?? "",
      label: value.label ?? value.Label ?? value.value ?? value.Value ?? "",
      active: value.active ?? value.Active ?? true
    };
  });
}

async function json(url, options = {}) {
  const response = await fetch(url, {
    headers: { "Content-Type": "application/json", ...(options.headers || {}) },
    ...options
  });
  const text = await response.text();
  const data = text ? JSON.parse(text) : {};
  if (!response.ok) {
    const message = data.message || data[0]?.message || response.statusText;
    const error = new Error(message);
    error.payload = data;
    throw error;
  }
  return data;
}

async function loadObjects() {
  const url = new URL(`${location.origin}${apiBase}/objects`);
  if (state.objectQuery) {
    url.searchParams.set("q", state.objectQuery);
  }
  const data = await json(url.pathname + url.search);
  state.objects = data.objects || [];
  if (!state.selectedObject && state.objects.length > 0) {
    state.selectedObject = objectName(state.objects[0]);
  }
}

async function selectObject(name) {
  state.selectedObject = name;
  state.selectedRecordId = "";
  state.editing = null;
  state.error = "";
  state.fieldErrors = [];
  state.query = "";
  render();
  await loadObjectDetail();
  await loadRecords();
  render();
}

async function loadObjectDetail() {
  if (!state.selectedObject) {
    state.objectDetail = null;
    return;
  }
  state.objectDetail = await json(`${apiBase}/objects/${encodeURIComponent(state.selectedObject)}`);
}

async function loadRecords() {
  if (!state.selectedObject) {
    state.records = [];
    state.recordTotal = 0;
    return;
  }
  const url = new URL(`${location.origin}${apiBase}/objects/${encodeURIComponent(state.selectedObject)}/records`);
  url.searchParams.set("limit", "50");
  url.searchParams.set("offset", "0");
  url.searchParams.set("includeDeleted", state.includeDeleted ? "true" : "false");
  if (state.query) {
    url.searchParams.set("q", state.query);
  }
  const data = await json(url.pathname + url.search);
  state.records = data.records || [];
  state.recordTotal = data.total ?? state.records.length;
}

async function openCreateDrawer() {
  if (!state.objectDetail) {
    await loadObjectDetail();
  }
  state.selectedRecordId = "";
  state.fieldErrors = [];
  state.error = "";
  state.editing = { mode: "create", record: null };
  render();
}

async function openEditDrawer(id) {
  if (!state.selectedObject || !id) {
    return;
  }
  state.selectedRecordId = id;
  state.fieldErrors = [];
  state.error = "";
  const record = await json(`${apiBase}/objects/${encodeURIComponent(state.selectedObject)}/records/${encodeURIComponent(id)}`);
  state.editing = { mode: "edit", record };
  render();
}

async function createRecord(objectNameValue, payload) {
  return json(`${apiBase}/objects/${encodeURIComponent(objectNameValue)}/records`, {
    method: "POST",
    body: JSON.stringify(payload)
  });
}

async function updateRecord(objectNameValue, id, payload) {
  return json(`${apiBase}/objects/${encodeURIComponent(objectNameValue)}/records/${encodeURIComponent(id)}`, {
    method: "PATCH",
    body: JSON.stringify(payload)
  });
}

async function deleteRecord(objectNameValue, id) {
  return json(`${apiBase}/objects/${encodeURIComponent(objectNameValue)}/records/${encodeURIComponent(id)}`, {
    method: "DELETE"
  });
}

async function undeleteRecord(objectNameValue, id) {
  return json(`${apiBase}/objects/${encodeURIComponent(objectNameValue)}/records/${encodeURIComponent(id)}/undelete`, {
    method: "POST"
  });
}

async function lookupSearch(objectNameValue, fieldName, term) {
  const url = new URL(`${location.origin}${apiBase}/lookup`);
  url.searchParams.set("object", objectNameValue);
  url.searchParams.set("field", fieldName);
  url.searchParams.set("q", term);
  return json(url.pathname + url.search);
}

async function saveDrawer() {
  if (!state.editing || !state.selectedObject) {
    return;
  }
  const form = root.querySelector(".dbm-form");
  if (!form) {
    return;
  }
  state.saving = true;
  state.error = "";
  state.fieldErrors = [];
  render();
  try {
    const fields = {};
    for (const field of state.objectDetail?.fields || []) {
      if (!fieldIsEditable(field)) {
        continue;
      }
      const name = getFieldName(field);
      const row = form.querySelector(`[data-field-row="${cssEscape(name)}"]`);
      if (!row) {
        continue;
      }
      const payload = fieldInputPayload(field, row);
      if (payload) {
        fields[name] = payload;
      }
    }
    if (state.editing.mode === "edit" && Object.keys(fields).length === 0) {
      state.editing = null;
      state.saving = false;
      render();
      return;
    }
    const payload = { fields };
    if (state.editing.mode === "create") {
      const result = await createRecord(state.selectedObject, payload);
      state.selectedRecordId = result.id || result.record?.id || "";
    } else {
      const id = state.editing.record?.id || state.selectedRecordId;
      await updateRecord(state.selectedObject, id, payload);
      state.selectedRecordId = id;
    }
    state.editing = null;
    await loadObjects();
    await loadRecords();
  } catch (error) {
    state.error = error.message;
    state.fieldErrors = error.payload?.fields || [];
  } finally {
    state.saving = false;
    render();
  }
}

async function deleteSelectedRecord(id) {
  if (!state.selectedObject || !id) {
    return;
  }
  await deleteRecord(state.selectedObject, id);
  state.editing = null;
  state.selectedRecordId = "";
  await loadObjects();
  await loadRecords();
  render();
}

async function undeleteSelectedRecord(id) {
  if (!state.selectedObject || !id) {
    return;
  }
  await undeleteRecord(state.selectedObject, id);
  await loadObjects();
  await loadRecords();
  if (state.editing?.record?.id === id) {
    await openEditDrawer(id);
  } else {
    render();
  }
}

function render() {
  const selected = state.objectDetail || state.objects.find((object) => objectName(object) === state.selectedObject);
  const title = selected ? objectLabel(selected) : "Local data";
  root.dataset.loading = "false";
  root.innerHTML = `
    <main class="dbm-shell">
      <header class="dbm-header">
        <div>
          <div class="dbm-kicker">DB Record Manager</div>
          <h1>${escapeHTML(title)}</h1>
        </div>
        <div class="dbm-status">
          <span>${escapeHTML(state.selectedObject || "No object selected")}</span>
          <span>${state.recordTotal} records</span>
        </div>
      </header>
      ${renderObjectRail()}
      <section class="dbm-main" aria-live="polite">
        ${state.error && !state.editing ? `<div class="dbm-alert">${escapeHTML(state.error)}</div>` : ""}
        ${renderRecordTable()}
      </section>
      ${renderRecordForm()}
    </main>
  `;
}

function renderObjectRail() {
  const query = normalizeName(state.objectQuery);
  const objects = state.objects.filter((object) => {
    const haystack = `${objectName(object)} ${objectLabel(object)} ${object.pluralLabel || ""}`;
    return normalizeName(haystack).includes(query);
  });
  const buttons = objects.map((object) => {
    const name = objectName(object);
    const selected = name === state.selectedObject;
    return `
      <button class="dbm-object" type="button" data-action="select-object" data-object="${attr(name)}" aria-selected="${selected}">
        <span>
          <strong>${escapeHTML(objectLabel(object))}</strong>
          <small>${escapeHTML(name)}</small>
        </span>
        <em>${escapeHTML(object.records ?? object.recordCount ?? 0)}</em>
      </button>
    `;
  }).join("");
  return `
    <aside class="dbm-rail">
      <label class="dbm-search">
        <span>Objects</span>
        <input type="search" data-action="object-search" value="${attr(state.objectQuery)}" placeholder="Search objects">
      </label>
      <div class="dbm-object-list">
        ${buttons || `<div class="dbm-empty">No matching objects</div>`}
      </div>
    </aside>
  `;
}

function renderRecordTable() {
  if (!state.selectedObject) {
    return `<div class="dbm-empty dbm-empty-large">Select an object to browse records.</div>`;
  }
  const fields = (state.objectDetail?.fields || []).filter((field) => getFieldName(field) !== "Id").slice(0, 6);
  const columnHeaders = fields.map((field) => `<th>${escapeHTML(getFieldLabel(field))}</th>`).join("");
  const deletedHeader = state.includeDeleted ? "<th>Deleted</th>" : "";
  const rows = state.records.map((record) => {
    const cells = fields.map((field) => {
      const formatted = formatCellValue(fieldValue(record, field));
      return `<td title="${attr(formatted)}">${escapeHTML(formatted)}</td>`;
    }).join("");
    const deleted = recordDeleted(record);
    const deletedCell = state.includeDeleted ? `<td>${deleted ? "Yes" : ""}</td>` : "";
    const title = recordTitle(record);
    return `
      <tr data-action="edit-record" data-record-id="${attr(record.id)}" aria-selected="${record.id === state.selectedRecordId}">
        <td title="${attr(title)}"><strong>${escapeHTML(title)}</strong></td>
        <td title="${attr(record.id || "")}"><code>${escapeHTML(record.id || "")}</code></td>
        ${cells}
        ${deletedCell}
      </tr>
    `;
  }).join("");
  return `
    <div class="dbm-toolbar">
      <div>
        <h2>${escapeHTML(state.objectDetail?.pluralLabel || state.objectDetail?.label || state.selectedObject)}</h2>
        <span>${state.recordTotal} visible records</span>
      </div>
      <label class="dbm-table-search">
        <span class="dbm-visually-hidden">Search records</span>
        <input type="search" data-action="record-search" value="${attr(state.query)}" placeholder="Search records">
      </label>
      <label class="dbm-check">
        <input type="checkbox" data-action="toggle-deleted" ${state.includeDeleted ? "checked" : ""}>
        Show deleted
      </label>
      <button type="button" class="dbm-icon-button" data-action="refresh" title="Refresh">&#8635;</button>
      <button type="button" class="dbm-primary" data-action="create-record">Create</button>
    </div>
    <div class="dbm-table-wrap">
      <table class="dbm-table">
        <thead>
          <tr>
            <th>Title</th>
            <th>Id</th>
            ${columnHeaders}
            ${deletedHeader}
          </tr>
        </thead>
        <tbody>
          ${rows || `<tr><td colspan="${fields.length + 2 + (state.includeDeleted ? 1 : 0)}"><div class="dbm-empty">No records found. Create a record or adjust the search.</div></td></tr>`}
        </tbody>
      </table>
    </div>
  `;
}

function renderRecordForm() {
  if (!state.editing) {
    return "";
  }
  const detail = state.objectDetail;
  const record = state.editing.record;
  const isCreate = state.editing.mode === "create";
  const title = isCreate ? `New ${detail?.label || state.selectedObject}` : recordTitle(record);
  const deleted = recordDeleted(record);
  const fields = (detail?.fields || []).map((field) => renderFieldRow(field, record)).join("");
  return `
    <aside class="dbm-drawer" aria-label="Record form">
      <form class="dbm-form">
        <div class="dbm-drawer-header">
          <div>
            <span>${isCreate ? "Create" : "Edit"}</span>
            <h2>${escapeHTML(title)}</h2>
          </div>
          <button type="button" class="dbm-icon-button" data-action="close-drawer" title="Close">&times;</button>
        </div>
        <div class="dbm-field-list">${fields}</div>
        <div class="dbm-drawer-footer">
          ${state.error ? `<div class="dbm-form-error">${escapeHTML(state.error)}</div>` : ""}
          ${!isCreate && deleted ? `<button type="button" class="dbm-secondary" data-action="undelete-record" data-record-id="${attr(record.id)}">Undelete</button>` : ""}
          ${!isCreate && !deleted ? `<button type="button" class="dbm-danger" data-action="delete-record" data-record-id="${attr(record.id)}">Delete</button>` : ""}
          <button type="button" class="dbm-secondary" data-action="close-drawer">Cancel</button>
          <button type="button" class="dbm-primary" data-action="save-record" ${state.saving ? "disabled" : ""}>${state.saving ? "Saving" : "Save"}</button>
        </div>
      </form>
    </aside>
  `;
}

function renderFieldRow(field, record) {
  const name = getFieldName(field);
  const control = getFieldControl(field);
  const value = fieldValue(record, field) ?? defaultFieldValue(field);
  const required = field.required || field.nillable === false;
  const editable = fieldIsEditable(field);
  const hasError = state.fieldErrors.some((fieldName) => normalizeName(fieldName) === normalizeName(name));
  return `
    <label class="dbm-field" data-field-row="${attr(name)}" data-control="${attr(control)}" data-error="${hasError}">
      <span>
        ${escapeHTML(getFieldLabel(field))}
        ${required ? `<strong title="Required">*</strong>` : ""}
        <small>${escapeHTML(name)}</small>
      </span>
      ${renderFieldControl(field, value, editable)}
    </label>
  `;
}

function renderFieldControl(field, value, editable) {
  const name = getFieldName(field);
  const control = editable ? getFieldControl(field) : "readonly";
  switch (control) {
  case "textarea":
    return `<textarea data-field="${attr(name)}">${escapeHTML(value)}</textarea>`;
  case "number":
    return `<input data-field="${attr(name)}" inputmode="decimal" value="${attr(value)}">`;
  case "checkbox":
    return `<input data-field="${attr(name)}" type="checkbox" ${truthy(value) ? "checked" : ""}>`;
  case "date":
    return `<input data-field="${attr(name)}" type="date" value="${attr(dateValue(value))}">`;
  case "datetime":
  case "datetime-local":
    return `<input data-field="${attr(name)}" type="datetime-local" value="${attr(datetimeValue(value))}">`;
  case "picklist":
    return renderPicklist(field, value);
  case "multipicklist":
    return renderMultiPicklist(field, value);
  case "lookup":
    return renderLookup(field, value);
  case "readonly":
    return `<output>${escapeHTML(formatCellValue(value))}</output>`;
  case "text":
  default:
    return `<input data-field="${attr(name)}" type="text" value="${attr(value)}">`;
  }
}

function renderPicklist(field, value) {
  const options = picklistValues(field).map((option) => `
    <option value="${attr(option.value)}" ${String(value) === String(option.value) ? "selected" : ""}>
      ${escapeHTML(option.active ? option.label : `Inactive: ${option.label}`)}
    </option>
  `).join("");
  const hasExisting = value !== "" && !picklistValues(field).some((option) => String(option.value) === String(value));
  return `
    <select data-field="${attr(getFieldName(field))}">
      <option value=""></option>
      ${hasExisting ? `<option value="${attr(value)}" selected>Inactive: ${escapeHTML(value)}</option>` : ""}
      ${options}
    </select>
  `;
}

function renderMultiPicklist(field, value) {
  const selected = new Set(Array.isArray(value) ? value : String(value || "").split(";").filter(Boolean));
  const options = picklistValues(field).map((option) => `
    <label class="dbm-pick-option">
      <input type="checkbox" data-field="${attr(getFieldName(field))}" value="${attr(option.value)}" ${selected.has(String(option.value)) ? "checked" : ""}>
      <span>${escapeHTML(option.active ? option.label : `Inactive: ${option.label}`)}</span>
    </label>
  `).join("");
  return `<div class="dbm-multi">${options || `<div class="dbm-empty">No values</div>`}</div>`;
}

function renderLookup(field, value) {
  const id = typeof value === "object" && value !== null ? value.id || value.Id || "" : value;
  const label = typeof value === "object" && value !== null ? value.title || value.name || id : id;
  return `
    <div class="dbm-lookup" data-lookup-field="${attr(getFieldName(field))}">
      <input data-field="${attr(getFieldName(field))}" data-value="${attr(id)}" value="${attr(label)}" placeholder="Search records">
      <div class="dbm-lookup-menu" hidden></div>
    </div>
  `;
}

function fieldInputPayload(field, row) {
  if (row.dataset.dirty !== "true") {
    return null;
  }
  const control = getFieldControl(field);
  if (control === "readonly") {
    return null;
  }
  if (control === "multipicklist") {
    const values = [...row.querySelectorAll("input[type='checkbox']:checked")].map((input) => input.value);
    return values.length ? { state: "value", values } : { state: "null" };
  }
  if (control === "checkbox") {
    return { state: "value", value: Boolean(row.querySelector("input")?.checked) };
  }
  if (control === "lookup") {
    const input = row.querySelector("input[data-field]");
    const id = input?.dataset.value || input?.value || "";
    return id ? { state: "value", id } : { state: "null" };
  }
  const element = row.querySelector("[data-field]");
  const value = element?.value ?? "";
  return value === "" ? { state: "null" } : { state: "value", value };
}

function defaultFieldValue(field) {
  const defaultValue = field.defaultValue || field.default_value;
  if (!defaultValue) {
    return "";
  }
  if (defaultValue.id) {
    return defaultValue.id;
  }
  if (defaultValue.values) {
    return defaultValue.values.join(";");
  }
  return defaultValue.value ?? "";
}

function formatCellValue(value) {
  if (value === null || value === undefined || value === "") {
    return "";
  }
  if (Array.isArray(value)) {
    return value.join("; ");
  }
  if (typeof value === "object") {
    return value.title || value.name || value.id || JSON.stringify(value);
  }
  if (typeof value === "boolean") {
    return value ? "True" : "False";
  }
  return value;
}

function truthy(value) {
  return value === true || value === "true" || value === "True" || value === 1 || value === "1";
}

function dateValue(value) {
  return String(value || "").slice(0, 10);
}

function datetimeValue(value) {
  return String(value || "").replace("Z", "").slice(0, 16);
}

function schedule(key, callback) {
  clearTimeout(searchTimers.get(key));
  searchTimers.set(key, setTimeout(callback, 250));
}

root.addEventListener("click", async (event) => {
  const actionTarget = event.target.closest("[data-action]");
  if (!actionTarget) {
    return;
  }
  const action = actionTarget.dataset.action;
  try {
    if (action === "select-object") {
      await selectObject(actionTarget.dataset.object);
    } else if (action === "refresh") {
      await loadObjects();
      await loadRecords();
      render();
    } else if (action === "create-record") {
      await openCreateDrawer();
    } else if (action === "edit-record") {
      await openEditDrawer(actionTarget.dataset.recordId);
    } else if (action === "close-drawer") {
      state.editing = null;
      state.error = "";
      state.fieldErrors = [];
      render();
    } else if (action === "save-record") {
      await saveDrawer();
    } else if (action === "delete-record") {
      await deleteSelectedRecord(actionTarget.dataset.recordId);
    } else if (action === "undelete-record") {
      await undeleteSelectedRecord(actionTarget.dataset.recordId);
    }
  } catch (error) {
    state.error = error.message;
    render();
  }
});

root.addEventListener("input", (event) => {
  const target = event.target;
  const fieldRow = target.closest("[data-field-row]");
  if (fieldRow) {
    fieldRow.dataset.dirty = "true";
  }
  if (target.dataset.action === "object-search") {
    state.objectQuery = target.value;
    render();
    focusSearch("[data-action='object-search']", state.objectQuery);
  } else if (target.dataset.action === "record-search") {
    state.query = target.value;
    schedule("record-search", async () => {
      try {
        await loadRecords();
      } catch (error) {
        state.error = error.message;
      }
      render();
      focusSearch("[data-action='record-search']", state.query);
    });
  } else if (target.closest(".dbm-lookup")) {
    const lookup = target.closest(".dbm-lookup");
    const fieldName = lookup.dataset.lookupField;
    target.dataset.value = "";
    schedule(`lookup-${fieldName}`, async () => {
      const menu = lookup.querySelector(".dbm-lookup-menu");
      if (!target.value.trim()) {
        menu.hidden = true;
        menu.innerHTML = "";
        return;
      }
      try {
        const data = await lookupSearch(state.selectedObject, fieldName, target.value.trim());
        const rows = data.records || [];
        menu.innerHTML = rows.map((row) => `
          <button type="button" data-lookup-id="${attr(row.id)}" data-lookup-title="${attr(row.title || row.id)}">
            <strong>${escapeHTML(row.title || row.id)}</strong>
            <small>${escapeHTML(row.subtitle || row.object || "")}</small>
          </button>
        `).join("") || `<div class="dbm-empty">No matches</div>`;
        menu.hidden = false;
      } catch (error) {
        menu.innerHTML = `<div class="dbm-empty">${escapeHTML(error.message)}</div>`;
        menu.hidden = false;
      }
    });
  }
});

root.addEventListener("change", async (event) => {
  const fieldRow = event.target.closest("[data-field-row]");
  if (fieldRow) {
    fieldRow.dataset.dirty = "true";
  }
  if (event.target.dataset.action === "toggle-deleted") {
    state.includeDeleted = event.target.checked;
    try {
      await loadRecords();
    } catch (error) {
      state.error = error.message;
    }
    render();
  }
});

root.addEventListener("click", (event) => {
  const option = event.target.closest("[data-lookup-id]");
  if (!option) {
    return;
  }
  const lookup = option.closest(".dbm-lookup");
  const input = lookup.querySelector("input[data-field]");
  input.value = option.dataset.lookupTitle;
  input.dataset.value = option.dataset.lookupId;
  const fieldRow = option.closest("[data-field-row]");
  if (fieldRow) {
    fieldRow.dataset.dirty = "true";
  }
  lookup.querySelector(".dbm-lookup-menu").hidden = true;
});

async function boot() {
  render();
  await loadObjects();
  if (state.selectedObject) {
    await loadObjectDetail();
    await loadRecords();
  }
  render();
}

boot().catch((error) => {
  root.dataset.loading = "false";
  root.innerHTML = `<main class="dbm-error">${escapeHTML(error.message)}</main>`;
});
