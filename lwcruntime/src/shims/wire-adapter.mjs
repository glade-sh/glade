import {
  ldsCacheKey,
  readLDSCache,
  recordIdsFromBody,
  registerLDSAdapter,
  writeLDSCache,
} from "./lds-cache.mjs";

const LOCAL_CONTEXT_HEADER = "X-Glade-LWC-Context";

function wireValue(result) {
  if (result?.error) {
    return { error: result.error, data: undefined };
  }
  return { data: result.data, error: undefined };
}

function isPlainWireJSON(value) {
  if (!Array.isArray(value)) {
    const prototype = Object.getPrototypeOf(value);
    if (prototype !== Object.prototype && prototype !== null) {
      return false;
    }
  }
  return Reflect.ownKeys(value).every((key) => {
    const descriptor = Object.getOwnPropertyDescriptor(value, key);
    return descriptor && Object.prototype.hasOwnProperty.call(descriptor, "value");
  });
}

function cloneWireJSON(value, seen = new WeakMap(), freeze = true) {
  if (!value || typeof value !== "object" || !isPlainWireJSON(value)) {
    return value;
  }
  const array = Array.isArray(value);
  const prototype = Object.getPrototypeOf(value);
  if (seen.has(value)) {
    return seen.get(value);
  }
  const clone = array ? new Array(value.length) : Object.create(prototype);
  seen.set(value, clone);
  for (const key of Reflect.ownKeys(value)) {
    if (array && key === "length") {
      continue;
    }
    const descriptor = Object.getOwnPropertyDescriptor(value, key);
    Object.defineProperty(clone, key, {
      value: cloneWireJSON(descriptor.value, seen),
      enumerable: descriptor.enumerable,
      writable: true,
      configurable: true,
    });
  }
  return freeze ? Object.freeze(clone) : clone;
}

function immutableWireValue(value, adapter) {
  const clone = cloneWireJSON(value, new WeakMap(), false);
  attachRefresh(clone, adapter);
  return Object.freeze(clone);
}

function hasUndefined(value) {
  if (value === undefined) {
    return true;
  }
  if (!value || typeof value !== "object") {
    return false;
  }
  if (Array.isArray(value)) {
    return value.some((item) => hasUndefined(item));
  }
  return Object.values(value).some((item) => hasUndefined(item));
}

function assertObjectParams(params) {
  if (params == null) {
    return {};
  }
  if (typeof params !== "object" || Array.isArray(params)) {
    throw new TypeError("Apex params must be an object");
  }
  return params;
}

function sameWireData(left, right) {
  if (left === right) {
    return true;
  }
  if (!left || !right || typeof left !== "object" || typeof right !== "object"
      || !isPlainWireJSON(left) || !isPlainWireJSON(right)
      || Array.isArray(left) !== Array.isArray(right)) {
    return false;
  }
  if (Array.isArray(left) && left.length !== right.length) {
    return false;
  }
  const keys = Object.keys(left);
  return keys.length === Object.keys(right).length && keys.every((key) =>
    Object.prototype.hasOwnProperty.call(right, key) && sameWireData(left[key], right[key]));
}

function emitFetchWireValue(adapter, value, suppressUnchanged = false) {
  const previous = adapter.__lastEmittedValue;
  // Compare this wire's last delivery, not the cache another wire may have updated.
  if (suppressUnchanged && previous && adapter.__lastEmittedCacheKey === adapter.cacheKey
      && previous.error === undefined && value.error === undefined
      && sameWireData(previous.data, value.data)) {
    return;
  }
  adapter.__lastEmittedValue = value;
  adapter.__lastEmittedCacheKey = adapter.cacheKey;
  adapter.dataCallback(value);
}

function emitEmptyFetchWireValue(adapter) {
  if (adapter.__lastEmptyValue) {
    return adapter.__lastEmptyValue;
  }
  const value = immutableWireValue({ data: undefined, error: undefined }, adapter);
  adapter.__lastEmptyValue = value;
  emitFetchWireValue(adapter, value);
  return value;
}

export function createFetchWireAdapter(endpoint, mapBody) {
  function FetchWireAdapter(dataCallback) {
    this.dataCallback = dataCallback;
    this.config = null;
    this.pending = 0;
    this.body = null;
    this.cacheKey = "";
    this.recordIdSet = new Set();
    this.unregisterLDS = registerLDSAdapter(this);
    this.__lastEmptyValue = null;
    emitEmptyFetchWireValue(this);
  }
  FetchWireAdapter.prototype.connect = function connect() {
    if (!this.unregisterLDS) {
      this.unregisterLDS = registerLDSAdapter(this);
    }
    if (this.config) {
      this.update(this.config);
    }
  };
  FetchWireAdapter.prototype.disconnect = function disconnect() {
    this.pending += 1;
    if (this.unregisterLDS) {
      this.unregisterLDS();
      this.unregisterLDS = null;
    }
  };
  FetchWireAdapter.prototype.update = function update(config) {
    this.config = config;
    this.body = mapBody(config);
    const cacheKey = this.body && !hasUndefined(this.body) ? ldsCacheKey(endpoint, this.body) : "";
    // Retire the previous configuration even when no new fetch will start.
    if (cacheKey !== this.cacheKey) {
      this.pending += 1;
    }
    this.cacheKey = cacheKey;
    if (!this.cacheKey) {
      this.recordIdSet = new Set();
      const value = emitEmptyFetchWireValue(this);
      return Promise.resolve(value);
    }
    this.__lastEmptyValue = null;
    this.recordIdSet = recordIdsFromBody(this.body);
    return this.refresh();
  };
  FetchWireAdapter.prototype.recordIds = function recordIds() {
    return this.recordIdSet || new Set();
  };
  FetchWireAdapter.prototype.refresh = function refresh(options = {}) {
    if (!this.cacheKey || !this.body) {
      return Promise.resolve();
    }
    if (!options.force) {
      const cached = readLDSCache(this.cacheKey);
      if (cached) {
        this.__lastEmptyValue = null;
        emitRuntimeEvent({
          kind: "lds",
          label: endpoint,
          status: "cache-hit",
          detail: {
            endpoint,
            body: this.body,
            result: cached?.data,
          },
        });
        const value = immutableWireValue({ data: cached?.data, error: cached?.error }, this);
        emitFetchWireValue(this, value);
        return Promise.resolve(value);
      }
    }
    const ticket = ++this.pending;
    const started = nowMs();
    let responseStatus = 0;
    return fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(this.body),
    })
      .then((response) => {
        responseStatus = response.status || 0;
        return response.json();
      })
      .then((result) => {
        emitRuntimeEvent({
          kind: "network",
          label: endpoint,
          status: result?.error ? "error" : "success",
          detail: {
            endpoint,
            method: "POST",
            status: responseStatus,
            durationMs: elapsedMs(started),
          },
        });
        if (ticket !== this.pending) {
          return;
        }
        const value = immutableWireValue(wireValue(result), this);
        this.__lastEmptyValue = null;
        writeLDSCache(this.cacheKey, value);
        emitRuntimeEvent({
          kind: "lds",
          label: endpoint,
          status: result?.error ? "error" : "success",
          detail: {
            endpoint,
            body: this.body,
            result: result?.data,
            error: result?.error,
            durationMs: elapsedMs(started),
          },
        });
        emitFetchWireValue(this, value, options.suppressUnchanged);
        return value;
      })
      .catch((err) => {
        emitRuntimeEvent({
          kind: "network",
          label: endpoint,
          status: "error",
          detail: {
            endpoint,
            method: "POST",
            status: responseStatus || undefined,
            durationMs: elapsedMs(started),
            error: errorMessage(err),
          },
        });
        if (ticket !== this.pending) {
          return;
        }
        const value = immutableWireValue({ error: { message: String(err) }, data: undefined }, this);
        this.__lastEmptyValue = null;
        emitRuntimeEvent({
          kind: "lds",
          label: endpoint,
          status: "error",
          detail: {
            endpoint,
            body: this.body,
            error: errorMessage(err),
            durationMs: elapsedMs(started),
          },
        });
        emitFetchWireValue(this, value);
        return value;
      });
  };
  return FetchWireAdapter;
}

export function createApexWireAdapter(className, methodName, options = { cacheable: true }) {
  return createApexWireAdapterWithOptions(className, methodName, options);
}

export function createApexWireAdapterWithOptions(className, methodName, options = {}) {
  function ApexAdapterOrInvoker(input) {
    if (typeof input === "function") {
      this.dataCallback = input;
      this.config = null;
      this.pending = 0;
      this.cacheKey = "";
      return;
    }
    return invokeApex(className, methodName, input ?? {});
  }
  ApexAdapterOrInvoker.prototype.connect = function connect() {
    if (this.config) {
      this.update(this.config);
    }
  };
  ApexAdapterOrInvoker.prototype.disconnect = function disconnect() {
    this.pending += 1;
  };
  ApexAdapterOrInvoker.prototype.update = function update(config) {
    this.config = config;
    if (hasUndefined(config)) {
      return;
    }
    this.cacheKey = apexCacheKey(className, methodName, config ?? {}, localContextToken());
    if (options.cacheable) {
      const cached = readLDSCache(this.cacheKey);
      if (cached) {
        emitApexEvent(className, methodName, config ?? {}, "cache-hit", {
          result: cached?.data,
        });
        const value = immutableWireValue({ data: cached?.data, error: cached?.error }, this);
        this.dataCallback(value);
        return Promise.resolve(value);
      }
    }
    const ticket = ++this.pending;
    return invokeApex(className, methodName, config ?? {})
      .then((data) => {
        if (ticket !== this.pending) {
          return;
        }
        const value = immutableWireValue({ data, error: undefined }, this);
        if (options.cacheable) {
          writeLDSCache(this.cacheKey, value);
        }
        this.dataCallback(value);
        return value;
      })
      .catch((err) => {
        if (ticket !== this.pending) {
          return;
        }
        const value = immutableWireValue({ error: apexWireErrorValue(err), data: undefined }, this);
        this.dataCallback(value);
        return value;
      });
  };
  ApexAdapterOrInvoker.prototype.refresh = function refresh(options = {}) {
    if (!this.config || hasUndefined(this.config)) {
      return Promise.resolve();
    }
    const config = this.config;
    const cacheKey = this.cacheKey;
    return invokeApex(className, methodName, config ?? {})
      .then((data) => {
        const value = immutableWireValue({ data, error: undefined }, this);
        if (cacheKey) {
          writeLDSCache(cacheKey, value);
        }
        this.dataCallback(value);
        return value;
      })
      .catch((err) => {
        const value = immutableWireValue({ error: apexWireErrorValue(err), data: undefined }, this);
        this.dataCallback(value);
        return value;
      });
  };
  return ApexAdapterOrInvoker;
}

export function invokeApex(className, methodName, params) {
  let bodyParams;
  try {
    bodyParams = assertObjectParams(params);
  } catch (err) {
    return Promise.reject(err);
  }
  const started = nowMs();
  let responseStatus = 0;
  let networkRecorded = false;
  let apexRecorded = false;
  return fetch("/lightning/wire/apex", {
    method: "POST",
    headers: { "Content-Type": "application/json", ...localContextHeaders() },
    body: JSON.stringify({
      className,
      method: methodName,
      params: bodyParams,
    }),
  })
    .then((response) => {
      responseStatus = response.status || 0;
      return response.json();
    })
    .then((result) => {
      const durationMs = elapsedMs(started);
      networkRecorded = true;
      emitRuntimeEvent({
        kind: "network",
        label: "/lightning/wire/apex",
        status: result?.error ? "error" : "success",
        detail: {
          endpoint: "/lightning/wire/apex",
          method: "POST",
          status: responseStatus,
          durationMs,
        },
      });
      if (result?.error) {
        const body = result.error.body || result.error;
        const err = new Error(body.message || result.error.message || "Apex invocation failed");
        err.body = body;
        err.status = result.error.status;
        apexRecorded = true;
        emitApexEvent(className, methodName, bodyParams, "error", {
          durationMs,
          body,
          status: err.status,
        });
        throw err;
      }
      apexRecorded = true;
      emitApexEvent(className, methodName, bodyParams, "success", {
        durationMs,
        result: result?.data,
      });
      return result?.data;
    })
    .catch((err) => {
      const durationMs = elapsedMs(started);
      if (!networkRecorded) {
        emitRuntimeEvent({
          kind: "network",
          label: "/lightning/wire/apex",
          status: "error",
          detail: {
            endpoint: "/lightning/wire/apex",
            method: "POST",
            status: responseStatus || err?.status,
            durationMs,
            error: errorMessage(err),
          },
        });
      }
      if (!apexRecorded) {
        emitApexEvent(className, methodName, bodyParams, "error", {
          durationMs,
          body: err?.body,
          status: err?.status,
          error: errorMessage(err),
        });
      }
      throw err;
    });
}

export function createGetRecordWireAdapter() {
  return createFetchWireAdapter("/lightning/wire/getRecord", (config) => ({
    recordId: config?.recordId,
    fields: (config?.fields ?? []).map((field) => {
      if (field && typeof field === "object") {
        return `${field.objectApiName}.${field.fieldApiName}`;
      }
      return String(field);
    }),
    optionalFields: (config?.optionalFields ?? []).map((field) => {
      if (field && typeof field === "object") {
        return `${field.objectApiName}.${field.fieldApiName}`;
      }
      return String(field);
    }),
  }));
}

function apexCacheKey(className, methodName, params, context = "") {
  return ldsCacheKey("/lightning/wire/apex", { className, method: methodName, params, context });
}

function apexWireErrorValue(err) {
  if (!err) {
    return { message: "Apex invocation failed" };
  }
  if (err.body || err.status) {
    return {
      message: err.body?.message || err.message || "Apex invocation failed",
      body: err.body,
      status: err.status,
    };
  }
  return { message: String(err.message || err) };
}

function attachRefresh(value, adapter) {
  if (!value || typeof value !== "object") {
    return value;
  }
  Object.defineProperty(value, "refresh", {
    configurable: true,
    enumerable: false,
    value: (options) => adapter.refresh(options),
  });
  return value;
}

function localContextHeaders() {
  const token = localContextToken();
  if (!token) {
    return {};
  }
  return { [LOCAL_CONTEXT_HEADER]: token };
}

function localContextToken() {
  const envelope = localContextEnvelope();
  if (!envelope) {
    return "";
  }
  try {
    return encodeURIComponent(JSON.stringify(envelope));
  } catch (_err) {
    return "";
  }
}

function localContextEnvelope() {
  const context = readJSONScript("glade-lwc-context");
  const config = readJSONScript("glade-lightning-config");
  if (!context && !config) {
    return null;
  }
  const pageReference = config?.pageReference || {};
  return {
    url: localContextURL(context || {}, pageReference),
    context: context || {},
    pageReference,
  };
}

function readJSONScript(id) {
  if (typeof document === "undefined") {
    return null;
  }
  const node = document.getElementById(id);
  if (!node) {
    return null;
  }
  try {
    return JSON.parse(node.textContent || "{}");
  } catch (_err) {
    return null;
  }
}

function localContextURL(context, pageReference) {
  const attrs = pageReference?.attributes || {};
  const state = pageReference?.state || {};
  const recordId = attrs.recordId || context.recordId || "";
  const objectApiName = attrs.objectApiName || context.objectApiName || "";
  if (recordId) {
    const objectPath = objectApiName || "Record";
    return `/lwc/preview/record/${encodeURIComponent(objectPath)}/${encodeURIComponent(recordId)}${localQuery({
      ...state,
      id: recordId,
      recordId,
      objectApiName,
    })}`;
  }
  return `${window.location.pathname}${window.location.search}`;
}

function localQuery(values) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(values || {})) {
    if (value == null || value === "") {
      continue;
    }
    params.set(key, String(value));
  }
  const text = params.toString();
  return text ? `?${text}` : "";
}

function emitApexEvent(className, methodName, params, status, detail = {}) {
  emitRuntimeEvent({
    kind: "apex",
    label: `${className}.${methodName}`,
    status,
    detail: {
      className,
      method: methodName,
      params,
      ...detail,
    },
  });
}

function emitRuntimeEvent(detail) {
  if (typeof document === "undefined" || typeof CustomEvent === "undefined") {
    return;
  }
  defer(() => {
    try {
      document.dispatchEvent(new CustomEvent("glade:runtime-event", { detail }));
    } catch (_err) {
      // Runtime event collection must not affect wire behavior.
    }
  });
}

function nowMs() {
  if (typeof performance !== "undefined" && typeof performance.now === "function") {
    return performance.now();
  }
  return Date.now();
}

function elapsedMs(started) {
  return Math.round((nowMs() - started) * 100) / 100;
}

function errorMessage(err) {
  return err?.message || String(err || "");
}

function defer(callback) {
  if (typeof queueMicrotask === "function") {
    queueMicrotask(callback);
    return;
  }
  Promise.resolve().then(callback);
}
