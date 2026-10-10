import assert from "node:assert/strict";
import test from "node:test";

test("uiAppsApi exports conservative local app menu helpers", async () => {
  const module = await import("../src/lightning/uiAppsApi.mjs");

  assert.equal(typeof module.getNavItems, "function");
  assert.equal(typeof module.getAppMenuItems, "function");
  assert.equal(typeof module.getAppMenuItem, "function");
  assert.deepEqual(await module.getNavItems(), { navItems: [] });
  assert.deepEqual(await module.getAppMenuItems(), { appMenuItems: [] });
  assert.deepEqual(await module.getAppMenuItem({ appMenuItemId: "standard__Sales" }), {
    appMenuItem: null,
  });
});

test("uiListsApi exposes captured wire and mutation exports", async () => {
  // Native c_*_wire/reference rows back this surface. Runtime data DTOs are
  // asserted by TestL11SalesforceConformance, through the product server.
  const { readFile } = await import("node:fs/promises");
  const source = (await readFile(new URL("../src/lightning/uiListsApi.mjs", import.meta.url), "utf8"))
    .replace('"/lightning/shims/core/wire-adapter.js"',
      JSON.stringify(new URL("../src/shims/wire-adapter.mjs", import.meta.url).href));
  const module = await import("data:text/javascript;base64," + Buffer.from(source).toString("base64"));
  for (const name of ["getListInfoByName", "getListInfosByName", "getListInfosByObjectName",
    "getListRecordsByName", "getListPreferences", "createListInfo", "updateListInfoByName",
    "deleteListInfo", "updateListPreferences"]) {
    assert.equal(typeof module[name], "function", name);
  }
  // Exact native r_*_null errors for the four captured mutation exports.
  for (const operation of ["createListInfo", "updateListInfoByName", "deleteListInfo", "updateListPreferences"]) {
    await assert.rejects(module[operation](null), error =>
      error.name === "Error" && error.message === `Invalid config for "${operation}"`);
  }
});

test("graphql modules export tag helpers and empty local query results", async () => {
  const graphql = await import("../src/lightning/graphql.mjs");
  const uiGraphQLApi = await import("../src/lightning/uiGraphQLApi.mjs");
  const document = graphql.gql`query AccountList { uiapi { query { Account { edges { node { Id } } } } } }`;

  assert.equal(typeof graphql.gql, "function");
  assert.equal(typeof graphql.graphql, "function");
  assert.equal(typeof graphql.query, "function");
  assert.equal(document.kind, "Document");
  assert.match(document.source, /AccountList/);
  assert.deepEqual(await graphql.graphql({ query: document }), { data: {}, errors: [] });
  assert.equal(typeof uiGraphQLApi.gql, "function");
  assert.equal(typeof uiGraphQLApi.graphql, "function");
  assert.equal(typeof uiGraphQLApi.query, "function");
  assert.deepEqual(await uiGraphQLApi.query(document), { data: {}, errors: [] });
});
