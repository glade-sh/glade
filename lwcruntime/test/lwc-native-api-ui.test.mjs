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

test("uiListsApi exports conservative local list helpers", async () => {
  const module = await import("../src/lightning/uiListsApi.mjs");

  assert.equal(typeof module.getListInfosByObjectName, "function");
  assert.equal(typeof module.getListInfoByName, "function");
  assert.equal(typeof module.getListRecordsByName, "function");
  assert.equal(typeof module.getListPreferences, "function");
  assert.equal(typeof module.updateListInfoByName, "function");
  assert.deepEqual(await module.getListInfosByObjectName({ objectApiName: "Account" }), {
    count: 1,
    currentPageToken: null,
    listInfos: [{
      displayColumns: [],
      filteredByInfo: [],
      label: "All",
      listReference: { objectApiName: "Account", listViewApiName: "All" },
      orderBy: [],
      scope: null,
      visibility: "Public",
    }],
    nextPageToken: null,
    previousPageToken: null,
  });
  const listAdapter = new module.getListInfosByObjectName((payload) => {
    listAdapter.payload = payload;
  });
  listAdapter.update({ objectApiName: "Contact" });
  assert.equal(listAdapter.payload.error, undefined);
  assert.deepEqual(listAdapter.payload.data.listInfos[0].listReference, {
    objectApiName: "Contact",
    listViewApiName: "All",
  });
  for (const [name, expected] of [
    ["getListInfoByName", {
      displayColumns: [],
      filteredByInfo: [],
      label: "AllContacts",
      listReference: { objectApiName: "Contact", listViewApiName: "AllContacts" },
      orderBy: [],
      scope: null,
      visibility: "Public",
    }],
    ["getListRecordsByName", {
      count: 0,
      currentPageToken: null,
      nextPageToken: null,
      previousPageToken: null,
      records: [],
    }],
    ["getListPreferences", {
      columnWidths: {},
      wrapText: false,
    }],
  ]) {
    const adapter = new module[name]((payload) => {
      adapter.payload = payload;
    });
    adapter.update({ objectApiName: "Contact", listViewApiName: "AllContacts" });
    assert.equal(adapter.payload.error, undefined, name);
    assert.deepEqual(adapter.payload.data, expected, name);
  }
  assert.deepEqual(await module.getListInfoByName({ objectApiName: "Account", listViewApiName: "AllAccounts" }), {
    displayColumns: [],
    filteredByInfo: [],
    label: "AllAccounts",
    listReference: { objectApiName: "Account", listViewApiName: "AllAccounts" },
    orderBy: [],
    scope: null,
    visibility: "Public",
  });
  assert.deepEqual(await module.getListRecordsByName({ objectApiName: "Account", listViewApiName: "AllAccounts" }), {
    count: 0,
    currentPageToken: null,
    nextPageToken: null,
    previousPageToken: null,
    records: [],
  });
  assert.deepEqual(await module.getListPreferences({ objectApiName: "Account", listViewApiName: "AllAccounts" }), {
    columnWidths: {},
    wrapText: false,
  });
  await assert.rejects(
    module.updateListInfoByName({ objectApiName: "Account", listViewApiName: "AllAccounts" }),
    (err) => err?.body?.errorCode === "GLADELWC091",
  );
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
