import assert from "node:assert/strict";
import test from "node:test";
import { createFetchWireAdapter, createGetRecordWireAdapter } from "../src/shims/wire-adapter.mjs";
import { notifyRecordUpdateAvailable, refreshApex } from "../src/shims/lds-cache.mjs";

function tick() {
  return new Promise((resolve) => setTimeout(resolve, 0));
}

test("LDS notify and refreshApex re-emit matching record wires", async () => {
  const originalFetch = globalThis.fetch;
  const names = ["Acme", "Local Rename", "Manual Refresh"];
  const calls = [];
  globalThis.fetch = async (_url, options) => {
    calls.push(JSON.parse(options.body));
    const name = names[Math.min(calls.length - 1, names.length - 1)];
    return {
      async json() {
        return {
          data: {
            id: "001000000000001AAA",
            fields: {
              Name: { value: name, displayValue: name },
            },
          },
        };
      },
    };
  };

  try {
    const values = [];
    const Adapter = createGetRecordWireAdapter();
    const adapter = new Adapter((value) => values.push(value));

    await adapter.update({ recordId: "001000000000001AAA", fields: ["Account.Name"] });
    assert.equal(values.at(-1).data.fields.Name.value, "Acme");

    await notifyRecordUpdateAvailable([{ recordId: "001000000000001AAA" }]);
    assert.equal(values.at(-1).data.fields.Name.value, "Local Rename");

    await refreshApex(values.at(-1));
    assert.equal(values.at(-1).data.fields.Name.value, "Manual Refresh");
    assert.equal(calls.length, 3);
    assert.deepEqual(calls[0], { recordId: "001000000000001AAA", fields: ["Account.Name"], optionalFields: [] });

    adapter.disconnect();
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("getRecord wire sends optionalFields and keeps cache keys distinct", async () => {
  const originalFetch = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (_url, options) => {
    calls.push(JSON.parse(options.body));
    return {
      async json() {
        return { data: { id: calls.at(-1).recordId, fields: { Name: { value: String(calls.length) } } } };
      },
    };
  };

  try {
    const values = [];
    const Adapter = createGetRecordWireAdapter();
    const adapter = new Adapter((value) => values.push(value));

    await adapter.update({ recordId: "001000000000777AAA", fields: ["Account.Name"], optionalFields: ["Account.Phone"] });
    await adapter.update({ recordId: "001000000000777AAA", fields: ["Account.Name"], optionalFields: ["Account.Phone"] });
    await adapter.update({ recordId: "001000000000777AAA", fields: ["Account.Name"], optionalFields: ["Account.Website"] });

    assert.deepEqual(calls.map((call) => call.optionalFields), [["Account.Phone"], ["Account.Website"]]);
    const dataValues = values.filter((value) => value.data);
    assert.equal(dataValues.length, 3);
    assert.equal(dataValues[0].data.fields.Name.value, "1");
    assert.equal(dataValues[1].data.fields.Name.value, "1");
    assert.equal(dataValues[2].data.fields.Name.value, "2");
    adapter.disconnect();
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("fetch wire suppresses null mapped bodies", async () => {
  const originalFetch = globalThis.fetch;
  let calls = 0;
  globalThis.fetch = async () => {
    calls += 1;
    throw new Error("fetch should not run");
  };

  try {
    const values = [];
    const Adapter = createFetchWireAdapter("/lightning/wire/skip", () => null);
    const adapter = new Adapter((value) => values.push(value));

    await adapter.update({ objectApiName: "" });

    assert.equal(calls, 0);
    assert.equal(values.length, 1);
    assert.deepEqual(values[0], { data: undefined, error: undefined });
    assert.equal(typeof values[0].refresh, "function");
    adapter.disconnect();
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("LDS notify refreshes batch getRecords wires by recordIds", async () => {
  const originalFetch = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (_url, options) => {
    calls.push(JSON.parse(options.body));
    return {
      async json() {
        return { data: { results: [{ statusCode: 200, result: { id: "001000000000001AAA" } }] } };
      },
    };
  };

  try {
    const values = [];
    const Adapter = createFetchWireAdapter("/lightning/wire/getRecords", (config) => config);
    const adapter = new Adapter((value) => values.push(value));

    await adapter.update({
      records: [{ recordIds: ["001000000000001AAA"], fields: ["Account.Name"] }],
    });
    await notifyRecordUpdateAvailable([{ recordId: "001000000000001AAA" }]);

    assert.equal(values.filter((value) => value.data).length, 2);
    assert.equal(calls.length, 2);
    adapter.disconnect();
  } finally {
    globalThis.fetch = originalFetch;
  }
});

test("LDS notify refreshes related list wires by parentRecordId", async () => {
  const originalFetch = globalThis.fetch;
  const calls = [];
  globalThis.fetch = async (_url, options) => {
    calls.push(JSON.parse(options.body));
    return {
      async json() {
        return { data: { records: [], count: calls.length } };
      },
    };
  };

  try {
    const values = [];
    const Adapter = createFetchWireAdapter("/lightning/wire/getRelatedListRecords", (config) => config);
    const adapter = new Adapter((value) => values.push(value));

    await adapter.update({
      parentRecordId: "001000000000001AAA",
      relatedListId: "Contacts",
      fields: ["Contact.LastName"],
    });
    await notifyRecordUpdateAvailable([{ recordId: "001000000000001AAA" }]);

    assert.equal(values.filter((value) => value.data).length, 2);
    assert.equal(calls.length, 2);
    adapter.disconnect();
  } finally {
    globalThis.fetch = originalFetch;
  }
});
