import assert from "node:assert/strict";
import test from "node:test";
import { gql, graphql, query, refreshGraphQL } from "../src/lightning/graphql.mjs";
import { executeQuery } from "../src/lightning/graphql-query.mjs";

test("native LIKE escaped underscore matches its literal character", async t => {
  t.mock.method(globalThis, "fetch", async url => ({
    ok: true,
    json: async () => String(url).endsWith("/describe")
      ? { fields: [{ name: "Id", type: "id" }, { name: "Key__c", type: "string" }] }
      : { records: [{ Id: "a", Key__c: "L12_A" }, { Id: "b", Key__c: "L12_B" }] },
  }));
  for (const [pattern, expected] of [["L12\\_A", ["a"]], ["L12\\_B", ["b"]], ["L12\\_Z", []], ["L12\\%", []]]) {
    const literal = JSON.stringify(pattern);
    const document = gql(`query { uiapi { query { W27Probe__c(first: 5, where: { Key__c: { like: ${literal} } }) { edges { node { Id Key__c { value } } } } } } }`);
    const result = await executeQuery(document);
    assert.ok(result.data, JSON.stringify(result));
    assert.deepEqual(result.data.uiapi.query.W27Probe__c.edges.map(edge => edge.node.Id), expected, pattern);
  }
});

test("native LIKE escapes wildcards and backslash", async t => {
  const records = [
    { Id: "a", Label__c: "alpha_b" }, { Id: "b", Label__c: "alpha%b" },
    { Id: "c", Label__c: "alpha\\_b" }, { Id: "d", Label__c: "alphaxb" },
  ];
  t.mock.method(globalThis, "fetch", async url => ({
    ok: true,
    json: async () => String(url).endsWith("/describe")
      ? { fields: [{ name: "Id", type: "id" }, { name: "Label__c", type: "string" }] }
      : { records },
  }));
  for (const [pattern, expected] of [
    ["alpha\\_b", ["a"]], ["alpha\\%b", ["b"]],
    ["alpha\\\\_b", ["c"]], ["alpha\\\\b", []], ["alpha_b", ["a", "b", "d"]],
  ]) {
    const literal = JSON.stringify(pattern);
    const document = gql(`query { uiapi { query { W27Labels__c(first: 5, where: { Label__c: { like: ${literal} } }) { edges { node { Id Label__c { value } } } } } } }`);
    const result = await executeQuery(document);
    assert.ok(result.data, JSON.stringify(result));
    assert.deepEqual(result.data.uiapi.query.W27Labels__c.edges.map(edge => edge.node.Id), expected, pattern);
  }
});

// Supplemental route coverage: the native input is the captured wire query;
// imperative and refresh calls share the same local executor.
test("LIKE escapes use the same matcher on wire, refresh, and imperative routes", async t => {
  const records = [{ Id: "a", Key__c: "L12_A" }, { Id: "b", Key__c: "L12_B" }];
  t.mock.method(globalThis, "fetch", async url => ({
    ok: true,
    json: async () => String(url).endsWith("/describe")
      ? { fields: [{ name: "Id", type: "id" }, { name: "Key__c", type: "string" }] }
      : { records },
  }));
  const previous = Object.getOwnPropertyDescriptor(globalThis, "window");
  Object.defineProperty(globalThis, "window", { configurable: true, value: {} });
  t.after(() => previous ? Object.defineProperty(globalThis, "window", previous) : delete globalThis.window);
  const document = gql(`query Filter($pattern: String) { uiapi { query { W27Routes__c(first: 5, where: { and: [{ Key__c: { in: ["L12_A", "L12_B"] } }, { Key__c: { like: $pattern } }] }) { edges { node { Id } } } } } }`);
  const variables = { pattern: "L12\\_A" };
  const ids = result => result.data.uiapi.query.W27Routes__c.edges.map(edge => edge.node.Id);
  const values = [];
  const adapter = new graphql(value => values.push(value));
  adapter.connect();
  adapter.update({ query: document, variables });
  await adapter.pending;
  assert.deepEqual(ids(values.at(-1)), ["a"]);
  await refreshGraphQL(values.at(-1));
  assert.deepEqual(ids(values.at(-1)), ["a"]);
  assert.deepEqual(ids(await graphql({ query: document, variables })), ["a"]);
  assert.deepEqual(ids(await query(document, variables)), ["a"]);
  assert.deepEqual(ids(await query({ query: document, variables })), ["a"]);
  adapter.disconnect();
});
