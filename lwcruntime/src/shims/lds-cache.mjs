const ldsAdapters = new Set();
const ldsCache = new Map();

export function ldsCacheKey(endpoint, body) {
  return `${endpoint}:${stableStringify(body || {})}`;
}

export function readLDSCache(key) {
  return ldsCache.get(key);
}

export function writeLDSCache(key, value) {
  ldsCache.set(key, value);
  emitRuntimeEvent({
    kind: "lds",
    label: "writeLDSCache",
    status: "write",
    detail: { action: "write", key },
  });
}

export function registerLDSAdapter(adapter) {
  ldsAdapters.add(adapter);
  return () => {
    ldsAdapters.delete(adapter);
  };
}

export function notifyRecordUpdateAvailable(items = []) {
  const recordIds = normalizeRecordIds(items);
  emitRuntimeEvent({
    kind: "lds",
    label: "notifyRecordUpdateAvailable",
    detail: { recordIds: Array.from(recordIds) },
  });
  const refreshes = [];
  for (const adapter of Array.from(ldsAdapters)) {
    if (typeof adapter.refresh !== "function") {
      continue;
    }
    if (recordIds.size === 0 || adapterMatches(adapter, recordIds)) {
      refreshes.push(adapter.refresh({ force: true }));
    }
  }
  return Promise.all(refreshes).then(() => undefined);
}

export function getRecordNotifyChange(items = []) {
  return notifyRecordUpdateAvailable(items);
}

export function refreshApex(value) {
  if (value && typeof value.refresh === "function") {
    return value.refresh({ force: true });
  }
  return Promise.resolve(value);
}

export function recordIdsFromBody(body) {
  const ids = new Set();
  collectRecordIds(ids, body);
  return ids;
}

function adapterMatches(adapter, recordIds) {
  if (typeof adapter.recordIds !== "function") {
    return false;
  }
  for (const id of adapter.recordIds()) {
    if (recordIds.has(id)) {
      return true;
    }
  }
  return false;
}

function normalizeRecordIds(items) {
  const ids = new Set();
  for (const item of Array.isArray(items) ? items : [items]) {
    if (typeof item === "string" && item.trim() !== "") {
      ids.add(item.trim());
      continue;
    }
    if (item && typeof item === "object" && typeof item.recordId === "string" && item.recordId.trim() !== "") {
      ids.add(item.recordId.trim());
    }
  }
  return ids;
}

function collectRecordIds(ids, value) {
  if (!value || typeof value !== "object") {
    return;
  }
  if (typeof value.recordId === "string" && value.recordId.trim() !== "") {
    ids.add(value.recordId.trim());
  }
  if (typeof value.parentRecordId === "string" && value.parentRecordId.trim() !== "") {
    ids.add(value.parentRecordId.trim());
  }
  if (typeof value.parentRecordId === "string" && value.parentRecordId.trim() !== "") {
    ids.add(value.parentRecordId.trim());
  }
  if (Array.isArray(value.recordIds)) {
    for (const recordId of value.recordIds) {
      if (typeof recordId === "string" && recordId.trim() !== "") {
        ids.add(recordId.trim());
      }
    }
  }
  if (value.fields && typeof value.fields.Id === "string" && value.fields.Id.trim() !== "") {
    ids.add(value.fields.Id.trim());
  }
  if (Array.isArray(value)) {
    for (const item of value) {
      collectRecordIds(ids, item);
    }
    return;
  }
  for (const item of Object.values(value)) {
    collectRecordIds(ids, item);
  }
}

function stableStringify(value) {
  if (value === null || typeof value !== "object") {
    return JSON.stringify(value);
  }
  if (Array.isArray(value)) {
    return `[${value.map((item) => stableStringify(item)).join(",")}]`;
  }
  return `{${Object.keys(value)
    .sort()
    .map((key) => `${JSON.stringify(key)}:${stableStringify(value[key])}`)
    .join(",")}}`;
}

function emitRuntimeEvent(detail) {
  if (typeof document === "undefined" || typeof CustomEvent === "undefined") {
    return;
  }
  defer(() => {
    try {
      document.dispatchEvent(new CustomEvent("glade:runtime-event", { detail }));
    } catch (_err) {
      // Runtime event collection must not affect LDS behavior.
    }
  });
}

function defer(callback) {
  if (typeof queueMicrotask === "function") {
    queueMicrotask(callback);
    return;
  }
  Promise.resolve().then(callback);
}
