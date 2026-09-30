import assert from "node:assert/strict";
import test from "node:test";
import { createApexWireAdapter, createFetchWireAdapter, invokeApex } from "../src/shims/wire-adapter.mjs";
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
      "/lightning/wire/task-10-3-generic-config-shapes",
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
      "/lightning/wire/task-10-3-generic-server-error",
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
      "/lightning/wire/task104-immutable-fetch-data",
      (config) => ({ key: config.key }),
    ),
    config: { key: "fetch-data-case" },
  },
  {
    name: "cacheable Apex",
    hasInitial: false,
    createAdapter: () => createApexWireAdapter("Task104ImmutableData", "load", { cacheable: true }),
    config: { key: "apex-data-case" },
  },
];

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
      "/lightning/wire/task104-immutable-fetch-error",
      (config) => ({ key: config.key }),
    ),
    config: { key: "fetch-error-case" },
    result: () => ({ error: nestedWireError() }),
    body: (value) => value.error,
    sourceBody: (result) => result.error,
  },
  {
    name: "Apex",
    createAdapter: () => createApexWireAdapter("Task104ImmutableError", "load", { cacheable: false }),
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
    const result = await invokeApex("Task104Imperative", "load", {});
    assert.strictEqual(result, imperativeData);
    assert.equal(Object.isFrozen(result), false);
    result.nested.values[0] = "mutable";
    result.nested.values.push("still mutable");
    assert.deepEqual(result.nested.values, ["mutable", "still mutable"]);

    await assert.rejects(
      () => invokeApex("Task104Imperative", "load", {}),
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
