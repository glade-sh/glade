import assert from "node:assert/strict";
import test from "node:test";
import { createGetRecordWireAdapter } from "../src/shims/wire-adapter.mjs";

// Wire-configuration browser contracts, initially observed at API 67.
// https://developer.salesforce.com/docs/platform/lwc/guide/data-wire-service-about.html
// Local adapter interleavings, not compiler/browser or Salesforce parity proof.
const recordId = number => "001" + String(number).padStart(12, "0") + "AAA";
const config = id => ({ recordId: id, fields: ["Account.Name"] });
const nextTurn = () => new Promise(resolve => setImmediate(resolve));

function deferred() {
  let resolve;
  let reject;
  const promise = new Promise((done, fail) => { resolve = done; reject = fail; });
  return { promise, resolve, reject };
}

function response(id, revision = 1) {
  return {
    status: 200,
    async json() {
      return { data: { id, fields: { Name: { value: "revision-" + revision } } } };
    },
  };
}

function harness(t, fetch) {
  const originalFetch = globalThis.fetch;
  const adapters = [];
  globalThis.fetch = fetch;
  t.after(() => {
    for (const adapter of adapters) adapter.disconnect();
    globalThis.fetch = originalFetch;
  });
  return () => {
    const values = [];
    const Adapter = createGetRecordWireAdapter();
    const adapter = new Adapter(value => values.push(value));
    adapters.push(adapter);
    return { adapter, values };
  };
}

let caseNumber = 10100;
for (const transition of ["cache-hit", "undefined"]) {
  for (const outcome of ["success", "server-error", "transport-error"]) {
    const a = recordId(++caseNumber);
    const b = recordId(++caseNumber);
    test("obsolete fetch " + outcome + " is ignored after " + transition + " configuration", async t => {
      const oldResponse = deferred();
      const calls = [];
      const create = harness(t, async (_url, options) => {
        const body = JSON.parse(options.body);
        calls.push(body);
        if (body.recordId === b) return response(b);
        assert.equal(body.recordId, a);
        return oldResponse.promise;
      });
      if (transition === "cache-hit") {
        await create().adapter.update(config(b));
      }
      const { adapter, values } = create();
      const pending = adapter.update(config(a));
      const callsBeforeTransition = calls.length;
      await adapter.update(config(transition === "cache-hit" ? b : undefined));
      assert.equal(calls.length, callsBeforeTransition, "transition should not start another request");
      const selectedValue = values.at(-1);
      assert.equal(selectedValue.data?.id, transition === "cache-hit" ? b : undefined);
      assert.equal(selectedValue.error, undefined);
      const countAfterTransition = values.length;

      if (outcome === "success") {
        oldResponse.resolve(response(a));
      } else if (outcome === "server-error") {
        oldResponse.resolve({ status: 503, async json() { return { error: { message: "obsolete server error" } }; } });
      } else {
        oldResponse.reject(new Error("obsolete transport error"));
      }
      await pending;
      await nextTurn();
      assert.equal(values.length, countAfterTransition, "obsolete response must not emit");
      assert.equal(values.at(-1), selectedValue, "selected data/error must remain unchanged");
      if (transition === "cache-hit") {
        const reader = create();
        await reader.adapter.update(config(b));
        assert.equal(reader.values.at(-1).data?.id, b, "obsolete response must not corrupt the selected cache key");
        assert.equal(reader.values.at(-1).error, undefined);
        assert.equal(calls.length, callsBeforeTransition, "selected cache entry must remain usable");
      }
    });
  }
}

test("same-config cache hit preserves an in-flight forced refresh", async t => {
  const id = recordId(10201);
  const refreshed = deferred();
  let calls = 0;
  const create = harness(t, async () => ++calls === 1 ? response(id) : refreshed.promise);
  const { adapter, values } = create();
  await adapter.update(config(id));
  const pending = adapter.refresh({ force: true });
  // Equivalent configuration with a fresh object and different property order.
  await adapter.update({ fields: ["Account.Name"], recordId: id });
  assert.equal(calls, 2);
  assert.equal(values.at(-1).data.fields.Name.value, "revision-1");
  refreshed.resolve(response(id, 2));
  await pending;
  await nextTurn();
  assert.equal(values.at(-1).data.fields.Name.value, "revision-2");
  const reader = create();
  await reader.adapter.update(config(id));
  assert.equal(reader.values.at(-1).data.fields.Name.value, "revision-2");
  assert.equal(calls, 2);
});

test("same-config uncached requests still prefer the newer request", async t => {
  const id = recordId(10202);
  const requests = [deferred(), deferred()];
  let calls = 0;
  const create = harness(t, async () => requests[calls++].promise);
  const { adapter, values } = create();
  const first = adapter.update(config(id));
  const second = adapter.update(config(id));
  assert.equal(calls, 2);
  requests[1].resolve(response(id, 2));
  await second;
  const count = values.length;
  requests[0].resolve(response(id, 1));
  await first;
  await nextTurn();
  assert.equal(values.length, count);
  assert.equal(values.at(-1).data.fields.Name.value, "revision-2");
});

test("ordinary configuration changes and forced refresh still populate the right cache", async t => {
  const a = recordId(10203);
  const b = recordId(10204);
  const calls = [];
  const create = harness(t, async (_url, options) => {
    const body = JSON.parse(options.body);
    calls.push(body.recordId);
    return response(body.recordId, calls.length);
  });
  const { adapter, values } = create();
  await adapter.update(config(a));
  assert.equal(values.at(-1).data.id, a);
  await adapter.update(config(b));
  assert.equal(values.at(-1).data.id, b);
  await adapter.refresh({ force: true });
  assert.equal(values.at(-1).data.id, b);
  assert.equal(values.at(-1).data.fields.Name.value, "revision-3");
  const reader = create();
  await reader.adapter.update(config(b));
  assert.equal(reader.values.at(-1).data.id, b);
  assert.equal(reader.values.at(-1).data.fields.Name.value, "revision-3");
  assert.deepEqual(calls, [a, b, b]);
});
