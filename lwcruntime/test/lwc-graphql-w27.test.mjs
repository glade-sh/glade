import assert from "node:assert/strict";
import test from "node:test";
import { gql, graphql, query, refreshGraphQL } from "../src/lightning/graphql.mjs";
import { executeQuery, NO_WIRE_PROVISION } from "../src/lightning/graphql-query.mjs";

// Local resolver/adapter regressions, not additional native parity claims.
// The five labels and LIKE twins come from the source-bound W27 controls.
const records = [
  { Id: "a", Key__c: "A", Label__c: "a%b" },
  { Id: "b", Key__c: "B", Label__c: "a\\b" },
  { Id: "c", Key__c: "C", Label__c: "a\\_b" },
  { Id: "d", Key__c: "D", Label__c: "axb" },
  { Id: "p", Key__c: "P", Label__c: "a_b" },
];
function mockData(t) {
  t.mock.method(globalThis, "fetch", async url => ({
    ok: true,
    json: async () => String(url).endsWith("/describe") ? {
      fields: [{ name: "Id", type: "id" }, { name: "Key__c", type: "string" }, { name: "Label__c", type: "string" }],
    } : { records },
  }));
}
function browserContext(t) {
  // Exercise the browser imperative executor rather than the Node fallback.
  const previous = Object.getOwnPropertyDescriptor(globalThis, "window");
  Object.defineProperty(globalThis, "window", { configurable: true, value: {} });
  t.after(() => {
    if (previous) Object.defineProperty(globalThis, "window", previous);
    else delete globalThis.window;
  });
}
const selection = "edges { cursor node { Id Key__c { value } Label__c { value } } } pageInfo { startCursor endCursor hasNextPage hasPreviousPage }";
const cursorQuery = gql(`query Page($cursor: String) { uiapi { query { W27Probe__c(first: 2, after: $cursor) { ${selection} } } } }`);
const edgeIds = result => result.data.uiapi.query.W27Probe__c.edges.map(edge => edge.node.Id);

test("W27 negative first transport envelope is wire-only; imperative pagination error stays unchanged", async t => {
  mockData(t);
  browserContext(t);
  const variable = gql(`query Page($n: Int) { uiapi { query { W27Probe__c(first: $n) { ${selection} } } } }`);
  const expected = { data: undefined, errors: [{ body: { errorCode: "ILLEGAL_QUERY_PARAMETER_VALUE", id: "-1582122214", statusCode: 400 }, errorType: "fetchResponse", headers: {}, ok: false, status: 400, statusText: "Bad Request" }] };
  const inline = gql(`query { uiapi { query { W27Probe__c(first: -1) { ${selection} } } } }`);
  const values = [];
  const adapter = new graphql(value => values.push(value));
  adapter.connect();
  for (const [document, variables] of [[variable, { n: -1 }], [variable, { n: -2 }], [variable, { n: "-1" }], [inline, {}]]) {
    adapter.update({ query: document, variables });
    await adapter.pending;
    assert.deepEqual(values.at(-1), expected);
    const legacy = await executeQuery(document, variables);
    assert.equal(legacy.data, undefined);
    assert.equal(legacy.errors.length, 1);
    assert.equal(legacy.errors[0].errorType, "adapterError");
    assert.equal(legacy.errors[0].error.length, 1);
    assert.deepEqual(legacy.errors[0].error[0].extensions, { ErrorType: "DataFetchingException" });
    assert.deepEqual(legacy.errors[0].error[0].paths, ["uiapi", "query", "W27Probe__c"]);
    assert.equal(legacy.errors[0].error[0].message, "Exception while fetching data (/uiapi/query/W27Probe__c) : You have paginated maximum allowed number of records without using upperBound, to go further please provide upperBound");
    assert.deepEqual(await graphql({ query: document, variables }), legacy);
    assert.deepEqual(await query(document, variables), legacy);
    assert.deepEqual(await query({ query: document, variables }), legacy);
  }
  for (const n of [0, 1]) assert.deepEqual(edgeIds(await executeQuery(variable, { n })), ["a"]);
  for (const n of [1.5, true, [2], { value: 2 }]) {
    assert.equal((await executeQuery(variable, { n })).errors[0].errorType, "adapterError");
  }
});

test("W27 wire cursor attempts settle without publishing; imperative cursor errors stay intact", async t => {
  mockData(t);
  browserContext(t);
  const values = [];
  const adapter = new graphql(value => values.push(value));
  adapter.connect();
  for (const cursor of [42, false, ["cursor"], { cursor: "owned" }]) {
    assert.equal(await executeQuery(cursorQuery, { cursor }, undefined, true), NO_WIRE_PROVISION);
    adapter.update({ query: cursorQuery, variables: { cursor } });
    await adapter.pending;
    assert.equal(values.length, 0);
    const legacy = await executeQuery(cursorQuery, { cursor });
    assert.equal(legacy.errors[0].errorType, "adapterError");
    assert.deepEqual(legacy.errors[0].error[0].extensions, { ErrorType: "DataFetchingException" });
    assert.equal(legacy.errors[0].error[0].message, "Exception while fetching data (/uiapi/query/W27Probe__c) : Could not parse the cursor.");
    assert.deepEqual(await graphql({ query: cursorQuery, variables: { cursor } }), legacy);
    assert.deepEqual(await query(cursorQuery, { cursor }), legacy);
  }
  for (const cursor of ["", "OWNED_INVALID_CURSOR", "é水😀"]) {
    adapter.update({ query: cursorQuery, variables: { cursor } });
    await adapter.pending;
    assert.equal(values.at(-1).errors[0].errorType, "adapterError");
  }
  adapter.update({ query: cursorQuery, variables: { cursor: null } });
  await adapter.pending;
  assert.deepEqual(edgeIds(values.at(-1)), ["a", "b"]);
  adapter.update({ query: cursorQuery, variables: { cursor: btoa("v1:0") } });
  await adapter.pending;
  assert.deepEqual(edgeIds(values.at(-1)), ["b", "c"]);
  const previous = values.at(-1), count = values.length;
  adapter.update({ query: cursorQuery, variables: { cursor: 42 } });
  await adapter.pending;
  await refreshGraphQL(previous);
  assert.equal(values.length, count);
  adapter.update({ query: cursorQuery, variables: { cursor: null } });
  const stale = adapter.pending;
  adapter.update({ query: cursorQuery, variables: { cursor: false } });
  await Promise.all([stale, adapter.pending]);
  assert.equal(values.length, count);
  adapter.update({ query: cursorQuery, variables: { cursor: null } });
  adapter.disconnect();
  await adapter.pending;
  assert.equal(values.length, count);
});

test("W27 LIKE preserves literal backslashes for inline and variable patterns", async t => {
  mockData(t);
  const variable = gql(`query Filter($pattern: String) { uiapi { query { W27Probe__c(first: 5, where: { Label__c: { like: $pattern } }) { ${selection} } } } }`);
  const patterns = [
    ["a_b", ["a", "b", "d", "p"]], ["a%b", ["a", "b", "c", "d", "p"]],
    ["a\\_b", ["c"]], ["a\\%b", ["b", "c"]], ["a\\\\b", []], ["a\\\\_b", []],
  ];
  for (const [pattern, ids] of patterns) {
    const inline = gql(`query { uiapi { query { W27Probe__c(first: 5, where: { Label__c: { like: ${JSON.stringify(pattern)} } }) { ${selection} } } } }`);
    assert.deepEqual(edgeIds(await executeQuery(variable, { pattern })), ids);
    assert.deepEqual(edgeIds(await executeQuery(inline)), ids);
  }
  const equality = gql(`query Filter($value: String) { uiapi { query { W27Probe__c(first: 5, where: { Label__c: { eq: $value } }) { ${selection} } } } }`);
  assert.deepEqual(edgeIds(await executeQuery(equality, { value: ["cursor"] }, undefined, true)), []);
  for (const pattern of [null, ""]) assert.equal((await executeQuery(variable, { pattern })).errors[0].errorType, "adapterError");
});

test("uncaptured mixed connections and inline cursor types retain their previous error boundary", async t => {
  mockData(t);
  const inline = gql(`query { uiapi { query { W27Probe__c(first: 2, after: 42) { ${selection} } } } }`);
  assert.equal((await executeQuery(inline, {}, undefined, true)).errors[0].errorType, "adapterError");
  const mixed = gql(`query Page($cursor: String) { uiapi { query { a: W27Probe__c(first: 2, after: $cursor) { ${selection} } b: W27Probe__c(first: 2) { ${selection} } } } }`);
  assert.equal((await executeQuery(mixed, { cursor: 42 }, undefined, true)).errors[0].errorType, "adapterError");
  const nonnullable = gql(`query Page($cursor: String!) { uiapi { query { W27Probe__c(first: 2, after: $cursor) { ${selection} } } } }`);
  assert.equal((await executeQuery(nonnullable, { cursor: 42 }, undefined, true)).errors[0].errorType, "adapterError");
});
