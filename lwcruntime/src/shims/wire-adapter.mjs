function wireValue(result) {
  if (result?.error) {
    return { error: result.error, data: undefined };
  }
  return { data: result.data, error: undefined };
}

export function createFetchWireAdapter(endpoint, mapBody) {
  function FetchWireAdapter(dataCallback) {
    this.dataCallback = dataCallback;
    this.config = null;
    this.pending = 0;
  }
  FetchWireAdapter.prototype.connect = function connect() {
    if (this.config) {
      this.update(this.config);
    }
  };
  FetchWireAdapter.prototype.disconnect = function disconnect() {
    this.pending += 1;
  };
  FetchWireAdapter.prototype.update = function update(config) {
    this.config = config;
    const ticket = ++this.pending;
    fetch(endpoint, {
      method: "POST",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify(mapBody(config)),
    })
      .then((response) => response.json())
      .then((result) => {
        if (ticket !== this.pending) {
          return;
        }
        this.dataCallback(wireValue(result));
      })
      .catch((err) => {
        if (ticket !== this.pending) {
          return;
        }
        this.dataCallback({ error: { message: String(err) } });
      });
  };
  return FetchWireAdapter;
}

export function createApexWireAdapter(className, methodName) {
  function ApexAdapterOrInvoker(input) {
    if (typeof input === "function") {
      this.dataCallback = input;
      this.config = null;
      this.pending = 0;
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
    const ticket = ++this.pending;
    invokeApex(className, methodName, config ?? {})
      .then((data) => {
        if (ticket !== this.pending) {
          return;
        }
        this.dataCallback({ data, error: undefined });
      })
      .catch((err) => {
        if (ticket !== this.pending) {
          return;
        }
        this.dataCallback({ error: { message: String(err?.message || err) }, data: undefined });
      });
  };
  return ApexAdapterOrInvoker;
}

export function invokeApex(className, methodName, params) {
  return fetch("/lightning/wire/apex", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({
      className,
      method: methodName,
      params: params ?? {},
    }),
  })
    .then((response) => response.json())
    .then((result) => {
      if (result?.error) {
        const err = new Error(result.error.message || "Apex invocation failed");
        err.body = result.error;
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
  }));
}
