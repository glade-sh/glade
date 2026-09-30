import assert from "node:assert/strict";
import test from "node:test";
import { createGetRecordWireAdapter } from "../src/shims/wire-adapter.mjs";
import { notifyRecordUpdateAvailable } from "../src/shims/lds-cache.mjs";

// API67 source: reference-notify-record-update.md SHA256 412758342b2c8959c8a913725571b8eadbb4757171451074b229efc59a500b98.
// Contract: lwc-contract-sha256-6955e08c676d49cd158946927afa0676bee0a87faebf343f8855606ade2c01de.
// Local adapter checks only; Salesforce/browser parity requires separate evidence.
function harness(t, recordId) {
  const originalFetch = globalThis.fetch;
  let name = "Acme";
  let calls = 0;
  globalThis.fetch = async () => {
    calls++;
    const fetchedName = name;
    return {
      status: 200,
      async json() {
        return { data: { id: recordId, fields: { Name: { value: fetchedName } } } };
      },
    };
  };
  const values = [];
  const Adapter = createGetRecordWireAdapter();
  const adapter = new Adapter(value => { if (value.data) values.push(value); });
  t.after(() => {
    adapter.disconnect();
    globalThis.fetch = originalFetch;
  });
  return { adapter, values, setName: value => { name = value; }, calls: () => calls };
}

test("notify refreshes affected wires but does not re-emit unchanged data", async t => {
  const recordId = "001000000000841AAA";
  const { adapter, values, setName, calls } = harness(t, recordId);
  await adapter.update({ recordId, fields: ["Account.Name"] });
  assert.equal(values.length, 1);

  await notifyRecordUpdateAvailable([{ recordId }]);
  assert.equal(calls(), 2, "notification must refetch the affected record");
  assert.equal(values.length, 1, "unchanged record data must not re-emit");

  setName("Renamed");
  await notifyRecordUpdateAvailable([{ recordId }]);
  assert.equal(calls(), 3);
  assert.equal(values.length, 2, "changed data must arrive before notification completes");
  assert.equal(values.at(-1).data.fields.Name.value, "Renamed");
});

test("a reconnected wire receives later record notifications", async t => {
  const recordId = "001000000000842AAA";
  const { adapter, values, setName, calls } = harness(t, recordId);
  await adapter.update({ recordId, fields: ["Account.Name"] });
  adapter.disconnect();
  setName("While disconnected");
  await notifyRecordUpdateAvailable([{ recordId }]);
  assert.equal(calls(), 1, "disconnected adapter must not receive LDS notification");
  adapter.connect();
  const beforeNotify = values.length;
  setName("After reconnect");

  await notifyRecordUpdateAvailable([{ recordId }]);
  assert.equal(calls(), 2, "reconnected adapter must be registered for LDS notification");
  assert.equal(values.length, beforeNotify + 1);
  assert.equal(values.at(-1).data.fields.Name.value, "After reconnect");
});

test("notification updates every affected wire only after semantic data change", async t => {
  const recordId = "001000000000843AAA";
  const originalFetch = globalThis.fetch;
  let name = "Acme";
  let calls = 0;
  const adapters = [];
  globalThis.fetch = async () => {
    calls++;
    const fetchedName = name;
    const field = { value: fetchedName, displayValue: fetchedName };
    const fields = calls % 2 === 0 ? { Type: { value: "Customer" }, Name: field }
      : { Name: field, Type: { value: "Customer" } };
    return { status: 200, async json() { return { data: { id: recordId, fields } }; } };
  };
  t.after(() => {
    for (const adapter of adapters) adapter.disconnect();
    globalThis.fetch = originalFetch;
  });
  const Adapter = createGetRecordWireAdapter();
  const consumers = Array.from({ length: 2 }, () => {
    const values = [];
    const adapter = new Adapter(value => { if (value.data) values.push(value); });
    adapters.push(adapter);
    return { adapter, values };
  });
  for (const { adapter } of consumers) {
    await adapter.update({ recordId, fields: ["Account.Name", "Account.Type"] });
  }
  assert.deepEqual(consumers.map(({ values }) => values.length), [1, 1]);

  await notifyRecordUpdateAvailable([{ recordId }]);
  assert.equal(calls, 3, "both affected wires must refetch");
  assert.deepEqual(consumers.map(({ values }) => values.length), [1, 1], "key ordering is not a data change");

  name = "Renamed";
  await notifyRecordUpdateAvailable([{ recordId }]);
  assert.equal(calls, 5);
  assert.deepEqual(consumers.map(({ values }) => values.length), [2, 2]);
  assert.deepEqual(consumers.map(({ values }) => values.at(-1).data.fields.Name.value), ["Renamed", "Renamed"]);
});
