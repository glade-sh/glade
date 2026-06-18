import {
  ldsCacheKey,
  readLDSCache,
  recordIdsFromBody,
  registerLDSAdapter,
  writeLDSCache,
} from "./lds-cache.mjs";

function wireValue(result) {
  if (result?.error) {
    return { error: result.error, data: undefined };
  }
  return { data: result.data, error: undefined };
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

export function createFetchWireAdapter(endpoint, mapBody) {
  function FetchWireAdapter(dataCallback) {
    this.dataCallback = dataCallback;
    this.config = null;
    this.pending = 0;
    this.body = null;
    this.cacheKey = "";
    this.recordIdSet = new Set();
    this.unregisterLDS = registerLDSAdapter(this);
  }
  FetchWireAdapter.prototype.connect = function connect() {
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
    if (!this.body || hasUndefined(this.body)) {
      this.cacheKey = "";
      this.recordIdSet = new Set();
      return Promise.resolve();
    }
    this.cacheKey = ldsCacheKey(endpoint, this.body);
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
        this.dataCallback(cached);
        return Promise.resolve(cached);
      }
    }
    const ticket = ++this.pending;
    return fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(this.body),
    })
      .then((response) => response.json())
      .then((result) => {
        if (ticket !== this.pending) {
          return;
        }
        const value = attachRefresh(wireValue(result), this);
        writeLDSCache(this.cacheKey, value);
        this.dataCallback(value);
        return value;
      })
      .catch((err) => {
        if (ticket !== this.pending) {
          return;
        }
        const value = attachRefresh({ error: { message: String(err) } }, this);
        this.dataCallback(value);
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
    this.cacheKey = apexCacheKey(className, methodName, config ?? {});
    if (options.cacheable) {
      const cached = readLDSCache(this.cacheKey);
      if (cached) {
        this.dataCallback(cached);
        return Promise.resolve(cached);
      }
    }
    const ticket = ++this.pending;
    return invokeApex(className, methodName, config ?? {})
      .then((data) => {
        if (ticket !== this.pending) {
          return;
        }
        const value = attachRefresh({ data, error: undefined }, this);
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
        const value = attachRefresh({ error: apexWireErrorValue(err), data: undefined }, this);
        this.dataCallback(value);
        return value;
      });
  };
  ApexAdapterOrInvoker.prototype.refresh = function refresh(options = {}) {
    if (!this.config || hasUndefined(this.config)) {
      return Promise.resolve();
    }
    return invokeApex(className, methodName, this.config ?? {})
      .then((data) => {
        const value = attachRefresh({ data, error: undefined }, this);
        if (this.cacheKey) {
          writeLDSCache(this.cacheKey, value);
        }
        this.dataCallback(value);
        return value;
      })
      .catch((err) => {
        const value = attachRefresh({ error: apexWireErrorValue(err), data: undefined }, this);
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
  return fetch("/lightning/wire/apex", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      className,
      method: methodName,
      params: bodyParams,
    }),
  })
    .then((response) => response.json())
    .then((result) => {
      if (result?.error) {
        const body = result.error.body || result.error;
        const err = new Error(body.message || result.error.message || "Apex invocation failed");
        err.body = body;
        err.status = result.error.status;
        throw err;
      }
      return result?.data;
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

function apexCacheKey(className, methodName, params) {
  return ldsCacheKey("/lightning/wire/apex", { className, method: methodName, params });
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
