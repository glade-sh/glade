import {
  ldsCacheKey,
  readLDSCache,
  recordIdsFromBody,
  registerLDSAdapter,
  writeLDSCache,
} from "./lds-cache.mjs";

const LOCAL_CONTEXT_HEADER = "X-Glade-LWC-Context";

const readOnlyWireDataHandler = {
  set() { return false; },
  deleteProperty() { return false; },
  defineProperty() { return false; },
};

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

function cloneWireJSON(value, seen = new WeakMap(), freeze = true, readOnly = false) {
  if (!value || typeof value !== "object" || !isPlainWireJSON(value)) {
    return value;
  }
  const array = Array.isArray(value);
  const prototype = Object.getPrototypeOf(value);
  if (seen.has(value)) {
    return seen.get(value);
  }
  const clone = array ? new Array(value.length) : Object.create(prototype);
  const published = readOnly ? new Proxy(clone, readOnlyWireDataHandler) : clone;
  seen.set(value, published);
  for (const key of Reflect.ownKeys(value)) {
    if (array && key === "length") {
      continue;
    }
    const descriptor = Object.getOwnPropertyDescriptor(value, key);
    Object.defineProperty(clone, key, {
      // Only the envelope's data graph uses read-only proxies; other values
      // retain their existing frozen projection.
      value: cloneWireJSON(descriptor.value, seen, true, readOnly || (!freeze && key === "data")),
      enumerable: descriptor.enumerable,
      writable: true,
      configurable: true,
    });
  }
  if (freeze) Object.freeze(clone);
  return published;
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

export function createFetchWireAdapter(endpoint, mapBody, { retainOnInvalidConfig = false } = {}) {
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
      if (retainOnInvalidConfig) {
        return Promise.resolve(this.__lastEmittedValue);
      }
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
      this.initialValueEmitted = false;
      return;
    }
    return invokeApex(className, methodName, input ?? {});
  }
  ApexAdapterOrInvoker.prototype.connect = function connect() {
    // Provision the empty envelope before the component's first render, even
    // when undefined configuration prevents an Apex request.
    if (!this.initialValueEmitted) {
      this.initialValueEmitted = true;
      this.dataCallback(immutableWireValue({ data: undefined, error: undefined }, this));
    }
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
    const cacheKey = apexCacheKey(className, methodName, config ?? {}, localContextToken());
    // A cache hit for a different configuration must retire the old request,
    // just as starting a new request does.
    if (cacheKey !== this.cacheKey) {
      this.pending += 1;
    }
    this.cacheKey = cacheKey;
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
    return invokeApex(className, methodName, config ?? {}, true)
      .then((data) => {
        if (ticket !== this.pending) {
          return;
        }
        const value = immutableWireValue({ data, error: undefined }, this);
        // Suppression retains an already pending delivery, but that inactive
        // configuration must not populate the reusable storable cache.
        if (options.cacheable && !hasUndefined(this.config)) {
          writeLDSCache(cacheKey, value);
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
    return invokeApex(className, methodName, config ?? {}, true)
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

export function invokeApex(className, methodName, params, cacheable = false) {
  let bodyParams;
  try {
    bodyParams = assertObjectParams(params);
  } catch (err) {
    if (typeof params === "string") {
      // Captured scalar String params fail with the action transport envelope,
      // rather than a client TypeError (r_imperative_scalar_params).
      return Promise.reject(apexFetchResponse({
        name: "Error",
        message: "aura://ApexActionController.execute: ServerServiceImpl::unwrapAction Class Cast Exception: class java.lang.String cannot be cast to class java.util.Map (java.lang.String and java.util.Map are in module java.base of loader 'bootstrap')",
      }, 500));
    }
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
      ...(cacheable ? { cacheable: true } : {}),
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
        const err = apexFetchResponse(body, result.error.status || 500);
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
  return createFetchWireAdapter("/lightning/wire/getRecord", (config) => {
    // Null/undefined and empty/malformed ID captures remain pending.
    // Validate syntax only; valid IDs for other entity types still reach LDS.
    if (config?.recordId == null || (typeof config.recordId === "string" &&
        !/^(?:[a-zA-Z0-9]{15}|[a-zA-Z0-9]{18})$/.test(config.recordId))) {
      return undefined;
    }
    // Empty/null field selections do not fetch or publish an error. Layout
    // selections are a separate valid read path and need no explicit fields.
    const emptyFields = config.fields == null || (Array.isArray(config.fields) && config.fields.length === 0);
    const hasLayouts = Array.isArray(config.layoutTypes) && config.layoutTypes.length > 0;
    if (emptyFields && !hasLayouts) {
      return undefined;
    }
    return {
      recordId: config.recordId,
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
    };
  }, { retainOnInvalidConfig: true });
}

function apexCacheKey(className, methodName, params, context = "") {
  return ldsCacheKey("/lightning/wire/apex", { className, method: methodName, params, context });
}

function apexWireErrorValue(err) {
  if (!err) {
    return { message: "Apex invocation failed" };
  }
  if (err.errorType === "fetchResponse") {
    return err;
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

function apexFetchResponse(body, status) {
  const response = {
    body,
    errorType: "fetchResponse",
    headers: {},
    ok: false,
    status,
    statusText: status === 500 ? "Server Error" : "",
  };
  // Preserve message access for callers without adding an enumerable field to
  // Salesforce's observed error object, including an empty handled message.
  Object.defineProperty(response, "message", { value: body.message ?? "Apex invocation failed" });
  return response;
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
