import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import { repoRoot } from "./helpers.mjs";

globalThis.window = { __gladeDiagnostics: [] };

function installDocumentContext(context) {
  globalThis.document = {
    getElementById(id) {
      if (id !== "glade-lwc-context") {
        return null;
      }
      return { textContent: JSON.stringify(context) };
    },
  };
}

function clearDocumentContext() {
  delete globalThis.document;
}

async function importRuntimeShim(name) {
  const url = pathToFileURL(path.join(repoRoot, "lwcruntime/src/shims", name));
  url.search = `case=${Date.now()}-${Math.random()}`;
  return import(url.href);
}

test("site activeLanguages reads shell context with deterministic fallback", async () => {
  const site = await importRuntimeShim("site.mjs");

  clearDocumentContext();
  assert.deepEqual(site.readActiveLanguages(), [{ code: "en-US", label: "English", active: true }]);

  installDocumentContext({
    community: {
      activeLanguages: [
        { code: "fr-FR", label: "French", active: true },
        { language: "es-MX", name: "Spanish", active: false },
      ],
    },
  });
  assert.deepEqual(site.readActiveLanguages(), [
    { code: "fr-FR", label: "French", active: true },
    { code: "es-MX", label: "Spanish", active: false },
  ]);
});

test("community shim exposes expanded site and container values", async () => {
  const community = await importRuntimeShim("community.mjs");
  installDocumentContext({
    community: {
      site: "Partner_Portal",
      siteId: "0DM000000000001",
      networkId: "0DB000000000001",
      basePath: "/partners",
      name: "Partner Portal",
      url: "https://partners.example.test",
      lwr: true,
      aura: false,
    },
  });

  assert.equal(community.readCommunityValue("site", ""), "Partner_Portal");
  assert.equal(community.readCommunityValue("name", ""), "Partner Portal");
  assert.equal(community.readCommunityValue("url", ""), "https://partners.example.test");
  assert.equal(community.readCommunityValue("basePath", ""), "/partners");
  assert.equal(community.readCommunityContextQuiet().lwr, true);
  assert.equal(community.readCommunityContextQuiet().aura, false);
});

test("userPermission shim returns booleans from shell context and defaults false", async () => {
  const permission = await importRuntimeShim("user-permission.mjs");

  clearDocumentContext();
  assert.equal(permission.readUserPermission("ViewSetup"), false);

  installDocumentContext({
    userPermissions: {
      ViewSetup: true,
      ModifyAllData: false,
    },
  });
  assert.equal(permission.readUserPermission("ViewSetup"), true);
  assert.equal(permission.readUserPermission("ModifyAllData"), false);
  assert.equal(permission.readUserPermission("AuthorApex"), false);
});

test("apexContinuation shim returns promise-based simulated result", async () => {
  const continuation = await importRuntimeShim("apex-continuation.mjs");
  const result = await continuation.invokeContinuation("LongCallout", { accountId: "001000000000001AAA" });

  assert.equal(result.status, "simulated");
  assert.equal(result.supportTier, "supported-local-simulated");
  assert.equal(result.method, "LongCallout");
  assert.deepEqual(result.params, { accountId: "001000000000001AAA" });
  assert.equal(continuation.createContinuation("QueuedCallout").method, "QueuedCallout");
});
