import assert from "node:assert/strict";
import test from "node:test";
import { createApexWireAdapter, invokeApex } from "../src/shims/wire-adapter.mjs";

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
