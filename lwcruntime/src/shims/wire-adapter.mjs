function wireValue(result) {
  if (result?.error) {
    return { error: result.error, data: undefined };
  }
  return { data: result.data, error: undefined };
}

export function createFetchWireAdapter(endpoint, mapBody) {
  return class FetchWireAdapter {
    constructor(dataCallback) {
      this.dataCallback = dataCallback;
      this.config = null;
      this.pending = 0;
    }

    connect() {
      if (this.config) {
        this.update(this.config);
      }
    }

    disconnect() {
      this.pending += 1;
    }

    update(config) {
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
    }
  };
}

export function createApexWireAdapter(className, methodName) {
  return createFetchWireAdapter("/lightning/wire/apex", (config) => ({
    className,
    method: methodName,
    params: config ?? {},
  }));
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
