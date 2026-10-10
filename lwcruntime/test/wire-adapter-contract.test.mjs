import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { createApexWireAdapter, createFetchWireAdapter, createGetRecordWireAdapter, invokeApex } from "../src/shims/wire-adapter.mjs";
import { refreshApex } from "../src/shims/lds-cache.mjs";

function assertFrozenPlainJSON(value, seen = new WeakSet()) {
  if (!value || typeof value !== "object" || seen.has(value)) {
    return;
  }
  const array = Array.isArray(value);
  const prototype = Object.getPrototypeOf(value);
  if (!array && prototype !== Object.prototype && prototype !== null) {
    return;
  }
  seen.add(value);
  assert.ok(Object.isFrozen(value), "wire JSON object or array must be frozen");
  for (const key of Reflect.ownKeys(value)) {
    const descriptor = Object.getOwnPropertyDescriptor(value, key);
    if (descriptor && Object.prototype.hasOwnProperty.call(descriptor, "value")) {
      assertFrozenPlainJSON(descriptor.value, seen);
    }
  }
}

function assertFrozenWireEnvelope(value) {
  assert.ok(Object.isFrozen(value), "wire envelope must be frozen");
  const refreshDescriptor = Object.getOwnPropertyDescriptor(value, "refresh");
  assert.equal(refreshDescriptor?.enumerable, false, "refresh descriptor remains non-enumerable");
  assert.equal(typeof refreshDescriptor?.value, "function", "frozen emission retains its refresh handle");
  assertFrozenPlainJSON(value.data);
  assertFrozenPlainJSON(value.error);
}

function nestedWireData(revision) {
  const value = { nested: { records: [{ labels: ["wire", revision], value: `revision-${revision}` }] } };
  value.self = value;
  return value;
}

// L07 native r_property_undefined and r_function_undefined at API 59/67.
test("Apex wire connects with an empty envelope even while undefined config suppresses requests", async () => {
  const originalFetch = globalThis.fetch;
  let calls = 0;
  globalThis.fetch = async () => {
    calls += 1;
    throw new Error("undefined configuration must not fetch");
  };
  let adapter;
  try {
    const values = [];
    const Adapter = createApexWireAdapter("L07InitialEnvelope", "load");
    adapter = new Adapter((value) => values.push(value));
    adapter.connect();
    assert.deepEqual(values, [{ data: undefined, error: undefined }]);
    assertFrozenWireEnvelope(values[0]);

    await adapter.update({ recordId: undefined });
    adapter.disconnect();
    adapter.connect();
    await Promise.resolve();
    assert.equal(calls, 0);
    assert.equal(values.length, 1, "reconnecting must not provision another initial envelope");
  } finally {
    adapter?.disconnect();
    globalThis.fetch = originalFetch;
  }
});

test("apex wire suppresses undefined params but invokes null params", async () => {
  const calls = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (_url, options) => {
    calls.push(JSON.parse(options.body));
    return {
      async json() {
        return { data: "ok" };
      },
    };
  };
  try {
    const values = [];
    const Adapter = createApexWireAdapter("ItemCtrl", "getItems");
    const adapter = new Adapter((value) => values.push(value));
    adapter.update({ recordId: undefined });
    await Promise.resolve();
    assert.equal(calls.length, 0);

    adapter.update({ recordId: null });
    await new Promise((resolve) => setTimeout(resolve, 0));
    assert.equal(calls.length, 1);
    assert.deepEqual(calls[0].params, { recordId: null });
    assert.deepEqual(values[0], { data: "ok", error: undefined });
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("generic fetch wire emits initial value and handles changed, undefined, and null config", async () => {
  const originalFetch = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (_url, options) => {
    const body = JSON.parse(options.body);
    calls.push(body);
    return {
      status: 200,
      async json() {
        return { data: { request: calls.length } };
      },
    };
  };

  let adapter;
  try {
    const values = [];
    const Adapter = createFetchWireAdapter(
      "/lightning/wire/generic-config-shapes",
      (config) => ({ required: config.required, optional: config.optional }),
    );
    adapter = new Adapter((value) => values.push(value));

    assert.equal(values[0].data, undefined);
    assert.equal(values[0].error, undefined);

    await adapter.update({ required: "first", optional: "a" });
    await adapter.update({ required: "second", optional: "b" });
    assert.deepEqual(calls, [
      { required: "first", optional: "a" },
      { required: "second", optional: "b" },
    ]);
    assert.deepEqual(values.slice(1).map((value) => value.data), [{ request: 1 }, { request: 2 }]);
    assert.equal(values[2].error, undefined);

    const callsBeforeUndefined = calls.length;
    await adapter.update({ required: undefined, optional: "suppressed" });
    assert.equal(calls.length, callsBeforeUndefined);
    assert.equal(values.at(-1).data, undefined);
    assert.equal(values.at(-1).error, undefined);

    await adapter.update({ required: null, optional: "explicit-null" });
    assert.equal(calls.length, callsBeforeUndefined + 1);
    assert.deepEqual(calls.at(-1), { required: null, optional: "explicit-null" });
    assert.deepEqual(values.at(-1).data, { request: 3 });
  } finally {
    adapter?.disconnect();
    globalThis.fetch = originalFetch;
  }
});

// L07 native r_lds_{property,function}_{null,undefined}, API 59/67.
for (const recordId of [null, undefined]) {
  test(`getRecord suppresses an initial ${recordId} record ID`, async () => {
    const originalFetch = globalThis.fetch;
    let calls = 0;
    globalThis.fetch = async () => {
      calls += 1;
      throw new Error("suppressed getRecord configuration must not fetch");
    };
    let adapter;
    try {
      const values = [];
      const Adapter = createGetRecordWireAdapter();
      adapter = new Adapter((value) => values.push(value));
      adapter.connect();
      const initial = values[0];
      await adapter.update({ recordId, fields: ["Account.Name"] });
      await adapter.refresh({ force: true });
      assert.equal(calls, 0);
      assert.equal(values.length, 1);
      assert.equal(values[0], initial);
      assert.deepEqual(initial, { data: undefined, error: undefined });
      assertFrozenWireEnvelope(initial);
    } finally {
      adapter?.disconnect();
      globalThis.fetch = originalFetch;
    }
  });
}

const l09ReviewRows = JSON.parse(readFileSync(
  new URL("../../internal/lwc/compile/testdata/l07_review_adapter.json", import.meta.url), "utf8"));

// L09's read helper ignores empty envelopes and observes these native windows.
// All four run together so the malformed-ID 30s window does not multiply by API.
test("getRecord invalid configurations match the native pending observations", async () => {
  const originalFetch = globalThis.fetch;
  const calls = [];
  const adapters = [];
  globalThis.fetch = async (_url, options) => {
    calls.push(JSON.parse(options.body));
    throw new Error("native pending configuration must not fetch");
  };
  try {
    const pending = l09ReviewRows.filter((row) => row.pending);
    assert.equal(pending.length, 4);
    await Promise.all(pending.map(async (row) => {
      assert.equal(row.basis, "org");
      const values = [];
      const Adapter = createGetRecordWireAdapter();
      const adapter = new Adapter((value) => values.push(value));
      adapters.push(adapter);
      adapter.connect();
      const { recordId, fields, layoutTypes } = row.config;
      const emptySelection = Array.isArray(fields) && fields.length === 0 &&
        !(Array.isArray(layoutTypes) && layoutTypes.length);
      const bounded = recordId === null || recordId === undefined || recordId === "" ||
        fields === null || emptySelection;
      const windowMs = bounded ? 4000 : 30000;
      const timeout = emptySelection ? { noDataOrErrorWithinMs: 4000 } : bounded ?
        { noEmissionWithinMs: 4000 } : { noDataOrErrorWithinMs: 30000 };
      await adapter.update(row.config);
      await adapter.refresh({ force: true });
      await new Promise((resolve) => setTimeout(resolve, windowMs));
      const delivered = values.find((value) => value.data !== undefined || value.error !== undefined);
      const observed = delivered ? (delivered.error ? { error: delivered.error } : delivered.data) : timeout;
      const text = "JSON|" + JSON.stringify({ returned: observed });
      for (const api of ["59.0", "67.0"]) {
        assert.equal(text, row.expected[api], `${row.id} API ${api}: exact native pending text`);
      }
      assert.equal(values.length, 1, `${row.id}: no emission beyond the initial empty envelope`);
      assertFrozenWireEnvelope(values[0]);
    }));
    assert.equal(calls.length, 0, "pending selections must not request LDS records or errors");
  } finally {
    for (const adapter of adapters) adapter.disconnect();
    globalThis.fetch = originalFetch;
  }
});

// Request contracts for native successful neighbours, not DTO parity against
// this stub: required/optional fields, layouts, combined selection and 15-char ID.
test("getRecord suppression preserves the native successful selection paths", async () => {
  const originalFetch = globalThis.fetch;
  const calls = [];
  const adapters = [];
  globalThis.fetch = async (_url, options) => {
    const body = JSON.parse(options.body);
    calls.push(body);
    return { status: 200, async json() { return { data: { id: body.recordId } }; } };
  };
  try {
    const selected = l09ReviewRows.filter((row) => !row.pending);
    assert.equal(selected.length, 5);
    for (const row of selected) {
      const values = [];
      const Adapter = createGetRecordWireAdapter();
      const adapter = new Adapter((value) => values.push(value));
      adapters.push(adapter);
      adapter.connect();
      const before = calls.length;
      await adapter.update(row.config);
      assert.equal(calls.length, before + 1, `${row.id}: selection must still fetch`);
      assert.equal(calls.at(-1).recordId, row.config.recordId);
      assert.deepEqual(calls.at(-1).fields, row.config.fields ?? []);
      assert.deepEqual(calls.at(-1).optionalFields, row.config.optionalFields ?? []);
      assert.equal(values.at(-1).data.id, row.config.recordId);
    }
  } finally {
    for (const adapter of adapters) adapter.disconnect();
    globalThis.fetch = originalFetch;
  }
});

// L07 native r_lds_{property,function}_error_to_undefined, API 59/67.
test("undefined getRecord config retains prior data and retires a pending error", async () => {
  const originalFetch = globalThis.fetch;
  const recordId = "001000000070001AAA";
  const deletedId = "001000000070002AAA";
  const data = { id: recordId, fields: { Name: { value: "owned" } } };
  const calls = [];
  let complete;
  const pendingResponse = new Promise((resolve) => { complete = resolve; });
  globalThis.fetch = async (_url, options) => {
    const body = JSON.parse(options.body);
    calls.push(body);
    if (body.recordId === deletedId) return pendingResponse;
    assert.equal(body.recordId, recordId);
    return { status: 200, async json() { return { data }; } };
  };
  let adapter;
  try {
    const values = [];
    const Adapter = createGetRecordWireAdapter();
    adapter = new Adapter((value) => values.push(value));
    adapter.connect();
    await adapter.update({ recordId, fields: ["Account.Name"] });
    const selected = values.at(-1);
    const count = values.length;
    const pending = adapter.update({ recordId: deletedId, fields: ["Account.Name"] });
    await adapter.update({ recordId: undefined, fields: ["Account.Name"] });
    assert.equal(values.length, count, "suppression must not publish an empty result");
    assert.equal(values.at(-1), selected);

    complete({
      status: 404,
      async json() {
        return { error: { status: 404, body: { message: "The requested resource does not exist" } } };
      },
    });
    await pending;
    await adapter.refresh({ force: true });
    assert.deepEqual(values.map((value) => value.data), [undefined, data]);
    assert.equal(values.at(-1), selected, "the retired error must not replace prior data");
    assert.equal(values.at(-1).error, undefined);
    assert.deepEqual(calls.map((body) => body.recordId), [recordId, deletedId]);
  } finally {
    adapter?.disconnect();
    globalThis.fetch = originalFetch;
  }
});

test("generic fetch wire preserves server error body on the error channel", async () => {
  const originalFetch = globalThis.fetch;
  const errorBody = {
    message: "server rejected request",
    statusCode: 503,
    body: { exceptionType: "ServiceUnavailable", message: "try later" },
  };
  let requestBody;
  globalThis.fetch = async (_url, options) => {
    requestBody = JSON.parse(options.body);
    return {
      status: 503,
      async json() {
        return { error: errorBody };
      },
    };
  };

  let adapter;
  try {
    const values = [];
    const Adapter = createFetchWireAdapter(
      "/lightning/wire/generic-server-error",
      (config) => ({ recordId: config.recordId }),
    );
    adapter = new Adapter((value) => values.push(value));

    await adapter.update({ recordId: "001000000000001AAA" });

    assert.deepEqual(requestBody, { recordId: "001000000000001AAA" });
    assert.deepEqual(values.at(-1).error, errorBody);
    assert.equal(values.at(-1).data, undefined);
  } finally {
    adapter?.disconnect();
    globalThis.fetch = originalFetch;
  }
});

test("imperative apex rejects non-object params and exposes Salesforce error body", async () => {
  await assert.rejects(() => invokeApex("ItemCtrl", "getItems", ["bad"]), /Apex params must be an object/);

  const originalFetch = globalThis.fetch;
  globalThis.fetch = async () => ({
    async json() {
      return {
        error: {
          status: 500,
          body: {
            message: "boom",
            exceptionType: "System.QueryException",
            stackTrace: "Class.ItemCtrl: line 4",
          },
        },
      };
    },
  });
  try {
    await assert.rejects(
      async () => invokeApex("ItemCtrl", "getItems", {}),
      (err) => {
        assert.equal(err.message, "boom");
        assert.equal(err.status, 500);
        assert.equal(err.body.exceptionType, "System.QueryException");
        return true;
      },
    );
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("apex wire preserves Salesforce error body and status", async () => {
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async () => ({
    async json() {
      return {
        error: {
          status: 500,
          body: {
            message: "boom",
            exceptionType: "System.QueryException",
            stackTrace: "Class.ItemCtrl: line 4",
          },
        },
      };
    },
  });
  try {
    const values = [];
    const Adapter = createApexWireAdapter("ItemCtrl", "getItems");
    const adapter = new Adapter((value) => values.push(value));

    await adapter.update({});
    await new Promise((resolve) => setTimeout(resolve, 0));

    assert.equal(values[0].data, undefined);
    assert.equal(values[0].error.status, 500);
    assert.equal(values[0].error.body.message, "boom");
    assert.equal(values[0].error.body.exceptionType, "System.QueryException");
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("cacheable apex wires use stable param keys and refreshApex forces a fetch", async () => {
  const calls = [];
  const originalFetch = globalThis.fetch;
  globalThis.fetch = async (_url, options) => {
    calls.push(JSON.parse(options.body));
    return {
      async json() {
        return { data: { count: calls.length } };
      },
    };
  };
  try {
    const values = [];
    const Adapter = createApexWireAdapter("ItemCtrl", "getItems", { cacheable: true });
    const adapter = new Adapter((value) => values.push(value));

    await adapter.update({ b: 2, a: 1 });
    assert.equal(values.at(-1).data.count, 1);

    await adapter.update({ a: 1, b: 2 });
    assert.equal(values.at(-1).data.count, 1);
    assert.equal(calls.length, 1);

    await values.at(-1).refresh();
    assert.equal(values.at(-1).data.count, 2);
    assert.equal(calls.length, 2);
  } finally {
    globalThis.fetch = originalFetch;
  }
});

// L07 native r_{property,function}_{cache_return,error_then_value}, API 59/67.
// Delay the prior response to cover the shared cause independently of timing.
for (const outcome of ["data", "error"]) {
  test(`cached Apex config survives a late ${outcome} response from a prior config`, async () => {
    const originalFetch = globalThis.fetch;
    const calls = [];
    const adapters = [];
    const priorMarker = outcome === "data" ? "other" : "__THROW__";
    const selectedMarker = outcome === "data" ? "owned" : "good";
    let complete;
    const priorResponse = new Promise((resolve) => { complete = resolve; });
    globalThis.fetch = async (_url, options) => {
      const marker = JSON.parse(options.body).params.marker;
      calls.push(marker);
      if (marker === priorMarker) return priorResponse;
      assert.equal(marker, selectedMarker);
      return { status: 200, async json() { return { data: { marker } }; } };
    };
    const Adapter = createApexWireAdapter(`L07CachedConfig${outcome}`, "load");
    const create = () => {
      const values = [];
      const adapter = new Adapter((value) => values.push(value));
      adapters.push(adapter);
      adapter.connect();
      return { adapter, values };
    };
    try {
      await create().adapter.update({ marker: selectedMarker });
      const source = create();
      const pending = source.adapter.update({ marker: priorMarker });
      await source.adapter.update({ marker: selectedMarker });
      const selected = source.values.at(-1);
      const count = source.values.length;
      assert.deepEqual(selected.data, { marker: selectedMarker });

      complete({
        status: outcome === "data" ? 200 : 500,
        async json() {
          return outcome === "data"
            ? { data: { marker: priorMarker } }
            : { error: { status: 500, body: { message: "L07 controlled error" } } };
        },
      });
      await pending;
      assert.equal(source.values.length, count, "prior config must not emit after the cache hit");
      assert.equal(source.values.at(-1), selected);

      const reader = create();
      await reader.adapter.update({ marker: selectedMarker });
      assert.deepEqual(reader.values.at(-1).data, { marker: selectedMarker });
      assert.deepEqual(calls, [selectedMarker, priorMarker], "the selected cache entry must remain usable");
    } finally {
      for (const adapter of adapters) adapter.disconnect();
      globalThis.fetch = originalFetch;
    }
  });
}

// L07 native r_{property,function}_value_then_error, API 59/67.
test("cached Apex data is provisioned before a subsequent error", async () => {
  const originalFetch = globalThis.fetch;
  const calls = [];
  const adapters = [];
  globalThis.fetch = async (_url, options) => {
    const marker = JSON.parse(options.body).params.marker;
    calls.push(marker);
    if (marker === "__THROW__") {
      return { status: 500, async json() { return { error: { status: 500, body: { message: "L07 controlled error" } } }; } };
    }
    return { status: 200, async json() { return { data: { marker } }; } };
  };
  const Adapter = createApexWireAdapter("L07CachedValueThenError", "load");
  const create = (values) => {
    const adapter = new Adapter((value) => values.push(value));
    adapters.push(adapter);
    adapter.connect();
    return adapter;
  };
  try {
    await create([]).update({ marker: "good" });
    const values = [];
    const adapter = create(values);
    const cached = adapter.update({ marker: "good" });
    assert.deepEqual(values.map((value) => value.data), [undefined, { marker: "good" }]);
    await cached;
    await adapter.update({ marker: "__THROW__" });
    assert.deepEqual(values.map((value) => value.data), [undefined, { marker: "good" }, undefined]);
    assert.equal(values.at(-1).error.body.message, "L07 controlled error");
    assert.deepEqual(calls, ["good", "__THROW__"], "the cached data delivery must not fetch");
    assertFrozenWireEnvelope(values.at(-1));
  } finally {
    for (const adapter of adapters) adapter.disconnect();
    globalThis.fetch = originalFetch;
  }
});

// L07 native r_property_rapid_suppress followed by r_function_rapid, API 59/67.
test("undefined Apex config retains pending delivery without caching its inactive result", async () => {
  const originalFetch = globalThis.fetch;
  let complete;
  let calls = 0;
  const pendingResponse = new Promise((resolve) => { complete = resolve; });
  globalThis.fetch = async () => {
    calls += 1;
    return pendingResponse;
  };
  let adapter, reader;
  try {
    const values = [];
    const Adapter = createApexWireAdapter("L07PendingSuppression", "load");
    adapter = new Adapter((value) => values.push(value));
    adapter.connect();
    const pending = adapter.update({ marker: "rapid-a" });
    await adapter.update({ marker: undefined });
    complete({ status: 200, async json() { return { data: { marker: "rapid-a" } }; } });
    await pending;
    assert.equal(calls, 1);
    assert.deepEqual(values.map((value) => value.data), [undefined, { marker: "rapid-a" }]);
    assert.equal(values.at(-1).error, undefined);
    const later = [];
    reader = new Adapter((value) => later.push(value));
    await reader.update({ marker: "rapid-a" });
    assert.equal(calls, 2, "the suppressed configuration must not seed a later cache hit");
    assert.deepEqual(later.map((value) => value.data), [{ marker: "rapid-a" }]);
  } finally {
    adapter?.disconnect();
    reader?.disconnect();
    globalThis.fetch = originalFetch;
  }
});

test("cacheable apex wire cache hits emit Apex console events", async () => {
  const calls = [];
  const events = [];
  const originalFetch = globalThis.fetch;
  const originalDocument = globalThis.document;
  const originalCustomEvent = globalThis.CustomEvent;
  globalThis.fetch = async (_url, options) => {
    calls.push(JSON.parse(options.body));
    return {
      async json() {
        return { data: { count: calls.length } };
      },
    };
  };
  globalThis.CustomEvent = class CustomEvent {
    constructor(type, init = {}) {
      this.type = type;
      this.detail = init.detail;
    }
  };
  globalThis.document = {
    getElementById() {
      return null;
    },
    dispatchEvent(event) {
      if (event.type === "glade:runtime-event") {
        events.push(event.detail);
      }
    },
  };
  try {
    const values = [];
    const Adapter = createApexWireAdapter("EventCtrl", "load", { cacheable: true });
    const adapter = new Adapter((value) => values.push(value));

    await adapter.update({ accountId: "001EVENT0000001AAA" });
    await Promise.resolve();
    await adapter.update({ accountId: "001EVENT0000001AAA" });
    await Promise.resolve();

    assert.equal(calls.length, 1);
    assert.equal(values.at(-1).data.count, 1);
    const apexEvents = events.filter((event) => event.kind === "apex" && event.label === "EventCtrl.load");
    assert.deepEqual(apexEvents.map((event) => event.status), ["success", "cache-hit"]);
    assert.deepEqual(apexEvents.at(-1).detail.params, { accountId: "001EVENT0000001AAA" });
    assert.deepEqual(apexEvents.at(-1).detail.result, { count: 1 });
  } finally {
    globalThis.fetch = originalFetch;
    globalThis.document = originalDocument;
    globalThis.CustomEvent = originalCustomEvent;
  }
});

test("apex wires send local LWC context and cache by active record", async () => {
  const calls = [];
  let recordId = "001LOCAL0000001AAA";
  const originalFetch = globalThis.fetch;
  const originalDocument = globalThis.document;
  const originalWindow = globalThis.window;
  globalThis.document = {
    getElementById(id) {
      if (id === "glade-lwc-context") {
        return {
          textContent: JSON.stringify({
            kind: "recordPage",
            objectApiName: "Account",
            recordId,
          }),
        };
      }
      if (id === "glade-lightning-config") {
        return {
          textContent: JSON.stringify({
            pageReference: {
              type: "standard__recordPage",
              attributes: {
                objectApiName: "Account",
                recordId,
                actionName: "view",
              },
            },
          }),
        };
      }
      return null;
    },
  };
  globalThis.window = { location: { pathname: "/", search: "" } };
  globalThis.fetch = async (_url, options) => {
    calls.push({
      body: JSON.parse(options.body),
      headers: options.headers,
    });
    return {
      async json() {
        return { data: { count: calls.length } };
      },
    };
  };
  try {
    const values = [];
    const Adapter = createApexWireAdapter("ContextCtrl", "load", { cacheable: true });
    const adapter = new Adapter((value) => values.push(value));

    await adapter.update({});
    assert.equal(values.at(-1).data.count, 1);

    await adapter.update({});
    assert.equal(values.at(-1).data.count, 1);
    assert.equal(calls.length, 1);

    recordId = "001LOCAL0000002AAA";
    await adapter.update({});
    assert.equal(values.at(-1).data.count, 2);
    assert.equal(calls.length, 2);

    const firstContext = JSON.parse(decodeURIComponent(calls[0].headers["X-Glade-LWC-Context"]));
    const secondContext = JSON.parse(decodeURIComponent(calls[1].headers["X-Glade-LWC-Context"]));
    assert.equal(firstContext.context.recordId, "001LOCAL0000001AAA");
    assert.equal(secondContext.context.recordId, "001LOCAL0000002AAA");
    assert.match(secondContext.url, /\/lwc\/preview\/record\/Account\/001LOCAL0000002AAA/);
    assert.match(secondContext.url, /recordId=001LOCAL0000002AAA/);
  } finally {
    globalThis.fetch = originalFetch;
    globalThis.document = originalDocument;
    globalThis.window = originalWindow;
  }
});

const immutableDataWireScenarios = [
  {
    name: "fetch",
    hasInitial: true,
    createAdapter: () => createFetchWireAdapter(
      "/lightning/wire/immutable-fetch-data",
      (config) => ({ key: config.key }),
    ),
    config: { key: "fetch-data-case" },
  },
  {
    name: "cacheable Apex",
    hasInitial: false,
    createAdapter: () => createApexWireAdapter("ImmutableWireData", "load", { cacheable: true }),
    config: { key: "apex-data-case" },
  },
];

// L07 r_{property,function}_immutable_{assign,delete,nested,push,sort,define},
// native API 59/67. The JavaScript engine supplies the read-only proxy errors.
test("Apex wire data rejects captured mutations through read-only proxy traps", async () => {
  const originalFetch = globalThis.fetch;
  const producer = { marker: "owned", nested: { value: "owned" }, items: [1, 2], ordinal: 1 };
  globalThis.fetch = async () => ({ status: 200, async json() { return { data: producer }; } });
  let adapter;
  try {
    const values = [];
    const Adapter = createApexWireAdapter("L07ReadOnlyData", "load");
    adapter = new Adapter((value) => values.push(value));
    await adapter.update({ marker: "owned", ordinal: 1 });
    const data = values.at(-1).data;
    const mutations = [
      [() => { data.marker = "changed"; }, "'set' on proxy: trap returned falsish for property 'marker'"],
      [() => { delete data.marker; }, "'deleteProperty' on proxy: trap returned falsish for property 'marker'"],
      [() => { data.nested.value = "changed"; }, "'set' on proxy: trap returned falsish for property 'value'"],
      [() => { data.items.push(3); }, "'set' on proxy: trap returned falsish for property '2'"],
      [() => { data.items.sort((a, b) => b - a); }, "'set' on proxy: trap returned falsish for property '0'"],
      [() => { Object.defineProperty(data, "marker", { value: "changed" }); }, "'defineProperty' on proxy: trap returned falsish for property 'marker'"],
    ];
    for (const [mutate, message] of mutations) {
      assert.throws(mutate, { name: "TypeError", message });
      assert.deepEqual(data, producer, "rejected writes must preserve the wire data");
    }
    const copy = { ...data };
    copy.marker = "changed";
    assert.equal(copy.marker, "changed", "the captured shallow clone remains mutable");
    assert.equal(data.marker, "owned");
    assertFrozenWireEnvelope(values.at(-1));
    assert.equal(Object.isFrozen(producer), false);
  } finally {
    adapter?.disconnect();
    globalThis.fetch = originalFetch;
  }
});

for (const scenario of immutableDataWireScenarios) {
  test(`${scenario.name} wire recursively freezes data, cache hits, and refresh emissions`, async () => {
    const originalFetch = globalThis.fetch;
    const calls = [];
    const producerValues = [];
    globalThis.fetch = async (url, options) => {
      calls.push({ url: String(url), body: JSON.parse(options.body) });
      return {
        status: 200,
        async json() {
          const data = nestedWireData(calls.length);
          producerValues.push(data);
          return { data };
        },
      };
    };
    let firstAdapter;
    let cacheAdapter;
    try {
      const firstValues = [];
      firstAdapter = new (scenario.createAdapter())((value) => firstValues.push(value));
      if (scenario.hasInitial) {
        assertFrozenWireEnvelope(firstValues[0]);
        assert.equal(firstValues[0].data, undefined);
      }
      await firstAdapter.update(scenario.config);
      const firstValue = firstValues.at(-1);
      assertFrozenWireEnvelope(firstValue);
      assert.equal(firstValue.data.nested.records[0].value, "revision-1");
      assert.equal(firstValue.data.self, firstValue.data, "cyclic JSON-shaped data must freeze without recursion failure");
      assert.notStrictEqual(firstValue.data, producerValues[0], "wire publishes a frozen projection, not the producer object");
      assert.equal(Object.isFrozen(producerValues[0]), false, "producer data must remain mutable");
      assert.throws(() => firstValue.data.nested.records[0].labels.push("mutation"), TypeError);
      producerValues[0].nested.records[0].labels.push("producer-only");
      assert.deepEqual(firstValue.data.nested.records[0].labels, ["wire", 1], "producer mutation must not change the published copy");

      const cacheValues = [];
      cacheAdapter = new (scenario.createAdapter())((value) => cacheValues.push(value));
      await cacheAdapter.update(scenario.config);
      const cacheHit = cacheValues.at(-1);
      assert.equal(calls.length, 1, "cache hit must not fetch");
      assert.notStrictEqual(cacheHit, firstValue, "each adapter gets a fresh wire envelope");
      assert.notStrictEqual(cacheHit.data, firstValue.data, "cache delivery gets a fresh public data graph");
      assert.deepEqual(cacheHit.data.nested.records[0], firstValue.data.nested.records[0]);
      assertFrozenWireEnvelope(cacheHit);

      await refreshApex(cacheHit);
      const refreshed = cacheValues.at(-1);
      assert.equal(calls.length, 2, "refresh descriptor must force a new fetch");
      assert.notStrictEqual(refreshed, cacheHit);
      assert.notStrictEqual(refreshed.data, cacheHit.data, "refresh must publish a fresh reactive data value");
      assert.equal(refreshed.data.nested.records[0].value, "revision-2");
      assert.equal(Object.isFrozen(producerValues[1]), false, "refresh must not freeze the producer response");
      assertFrozenWireEnvelope(refreshed);
    } finally {
      firstAdapter?.disconnect();
      cacheAdapter?.disconnect();
      globalThis.fetch = originalFetch;
    }
  });
}

function nestedWireError() {
  const details = { items: ["denied", { codes: [403] }] };
  details.self = details;
  const error = { message: "wire error", details };
  error.self = error;
  return error;
}

const immutableErrorWireScenarios = [
  {
    name: "fetch",
    createAdapter: () => createFetchWireAdapter(
      "/lightning/wire/immutable-fetch-error",
      (config) => ({ key: config.key }),
    ),
    config: { key: "fetch-error-case" },
    result: () => ({ error: nestedWireError() }),
    body: (value) => value.error,
    sourceBody: (result) => result.error,
  },
  {
    name: "Apex",
    createAdapter: () => createApexWireAdapter("ImmutableWireError", "load", { cacheable: false }),
    config: { key: "apex-error-case" },
    result: () => ({ error: { status: 503, body: nestedWireError() } }),
    body: (value) => value.error.body,
    sourceBody: (result) => result.error.body,
  },
];

for (const scenario of immutableErrorWireScenarios) {
  test(`${scenario.name} wire freezes nested error bodies and arrays`, async () => {
    const originalFetch = globalThis.fetch;
    const producerErrors = [];
    globalThis.fetch = async (_url, options) => {
      JSON.parse(options.body);
      return {
        status: 503,
        async json() {
          const result = scenario.result();
          producerErrors.push(scenario.sourceBody(result));
          return result;
        },
      };
    };
    let adapter;
    try {
      const values = [];
      adapter = new (scenario.createAdapter())((value) => values.push(value));
      await adapter.update(scenario.config);
      const emitted = values.at(-1);
      assertFrozenWireEnvelope(emitted);
      assert.equal(emitted.data, undefined);
      const error = scenario.body(emitted);
      assert.equal(error.self, error, "cycle-safe freezing preserves self references");
      assert.ok(Object.isFrozen(error.details.items));
      assert.throws(() => error.details.items[1].codes.push(500), TypeError);
      assert.equal(error.message, "wire error");
      assert.equal(Object.isFrozen(producerErrors[0]), false, "wire emission must not freeze producer-owned errors");
      assert.equal(Object.isFrozen(producerErrors[0].details.items), false);
      producerErrors[0].details.items.push("producer-only");
      assert.equal(error.details.items.length, 2, "producer mutation must not change the published error copy");
    } finally {
      adapter?.disconnect();
      globalThis.fetch = originalFetch;
    }
  });
}

test("imperative invokeApex results and rejection bodies remain mutable", async () => {
  const originalFetch = globalThis.fetch;
  const imperativeData = { nested: { values: ["before"] } };
  const imperativeErrorBody = { message: "imperative error", detail: { values: ["before"] } };
  let calls = 0;
  globalThis.fetch = async () => ({
    status: 200,
    async json() {
      calls += 1;
      return calls === 1
        ? { data: imperativeData }
        : { error: { status: 400, body: imperativeErrorBody } };
    },
  });
  try {
    const result = await invokeApex("ImperativeMutable", "load", {});
    assert.strictEqual(result, imperativeData);
    assert.equal(Object.isFrozen(result), false);
    result.nested.values[0] = "mutable";
    result.nested.values.push("still mutable");
    assert.deepEqual(result.nested.values, ["mutable", "still mutable"]);

    await assert.rejects(
      () => invokeApex("ImperativeMutable", "load", {}),
      (error) => {
        assert.strictEqual(error.body, imperativeErrorBody);
        assert.equal(Object.isFrozen(error.body), false);
        error.body.detail.values.push("mutable error body");
        assert.deepEqual(error.body.detail.values, ["before", "mutable error body"]);
        return true;
      },
    );
  } finally {
    globalThis.fetch = originalFetch;
  }
});
