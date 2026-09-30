import assert from "node:assert/strict";
import test from "node:test";
import { createApexWireAdapter } from "../src/shims/wire-adapter.mjs";
import { refreshApex } from "../src/shims/lds-cache.mjs";

function deferred() {
  let resolve;
  const promise = new Promise((done) => { resolve = done; });
  return { promise, resolve };
}

function apexResponse(data) {
  return {
    status: 200,
    async json() {
      return { data };
    },
  };
}

function adapterFactory(t, className, fetchImpl) {
  const originalFetch = globalThis.fetch;
  const adapters = [];
  globalThis.fetch = fetchImpl;
  t.after(() => {
    for (const adapter of adapters) adapter.disconnect();
    globalThis.fetch = originalFetch;
  });
  return () => {
    const values = [];
    const Adapter = createApexWireAdapter(className, "load", { cacheable: true });
    const adapter = new Adapter((value) => values.push(value));
    adapters.push(adapter);
    return { adapter, values };
  };
}

test("refreshApex stays bound to its source config after wire reconfiguration", async (t) => {
  const calls = [];
  const delayedRefresh = deferred();
  let aCalls = 0;
  const create = adapterFactory(t, "Task1012RefreshLifetime", async (_url, options) => {
    const recordId = JSON.parse(options.body).params.recordId;
    calls.push(recordId);
    if (recordId === "A") {
      aCalls += 1;
      return aCalls === 1
        ? apexResponse({ recordId: "A", revision: 1 })
        : delayedRefresh.promise;
    }
    assert.equal(recordId, "B");
    return apexResponse({ recordId: "B", revision: 1 });
  });

  const bSeed = create();
  await bSeed.adapter.update({ recordId: "B" });
  const source = create();
  await source.adapter.update({ recordId: "A" });

  const pendingRefresh = refreshApex(source.values.at(-1));
  await source.adapter.update({ recordId: "B" });
  assert.equal(source.values.at(-1).data.recordId, "B");
  assert.deepEqual(calls, ["B", "A", "A"]);

  delayedRefresh.resolve(apexResponse({ recordId: "A", revision: 2 }));
  await pendingRefresh;

  const newBSubscriber = create();
  await newBSubscriber.adapter.update({ recordId: "B" });
  assert.deepEqual(newBSubscriber.values.at(-1).data, { recordId: "B", revision: 1 });
  assert.deepEqual(calls, ["B", "A", "A"], "the new B subscriber should receive B from cache");
});

test("same-config cache hit keeps a pending refresh on its original cache entry", async (t) => {
  const calls = [];
  const delayedRefresh = deferred();
  const create = adapterFactory(t, "Task1012SameConfigRefreshLifetime", async (_url, options) => {
    const recordId = JSON.parse(options.body).params.recordId;
    calls.push(recordId);
    return calls.length === 1
      ? apexResponse({ recordId, revision: 1 })
      : delayedRefresh.promise;
  });

  const source = create();
  await source.adapter.update({ recordId: "same" });
  const pendingRefresh = refreshApex(source.values.at(-1));
  await source.adapter.update({ recordId: "same" });
  assert.equal(source.values.at(-1).data.revision, 1);
  assert.deepEqual(calls, ["same", "same"]);

  delayedRefresh.resolve(apexResponse({ recordId: "same", revision: 2 }));
  await pendingRefresh;

  const newSubscriber = create();
  await newSubscriber.adapter.update({ recordId: "same" });
  assert.deepEqual(newSubscriber.values.at(-1).data, { recordId: "same", revision: 2 });
  assert.deepEqual(calls, ["same", "same"]);
});
