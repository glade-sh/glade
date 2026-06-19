import assert from "node:assert/strict";
import test from "node:test";

function installBrowserContext(context = {}) {
  globalThis.window = {
    location: { href: "http://localhost/lwc/preview/utility/Support_Utility" },
    __gladeDiagnostics: [],
  };
  globalThis.document = {
    dispatchEvent(event) {
      this.events.push(event);
    },
    events: [],
    getElementById(id) {
      if (id !== "glade-lwc-context") {
        return null;
      }
      return { textContent: JSON.stringify(context) };
    },
  };
  globalThis.CustomEvent = class CustomEvent extends Event {
    constructor(type, options = {}) {
      super(type);
      this.detail = options.detail;
    }
  };
}

test("platformUtilityBarApi simulates utility bar state and callbacks", async () => {
  installBrowserContext({
    workspace: {
      utilities: [{
        id: "support",
        label: "Support",
        componentName: "c:SupportUtility",
        icon: "utility:question",
      }],
    },
  });
  const module = await import("../src/lightning/platformUtilityBarApi.mjs");

  assert.equal(typeof module.EnclosingUtilityId, "function");
  assert.equal(await module.EnclosingUtilityId(), "support");
  assert.equal(await module.open("support", { autoFocus: true }), true);
  assert.equal(await module.focusUtility("support"), true);
  assert.equal(await module.updateUtility("support", { label: "Cases", icon: "utility:case", highlighted: true }), true);
  assert.equal(await module.setUtilityLabel({ utilityId: "support", label: "Cases" }), true);
  assert.equal(await module.setUtilityIcon({ utilityId: "support", icon: "utility:case" }), true);
  assert.equal(await module.setUtilityHighlighted({ utilityId: "support", highlighted: true }), true);

  let clicked = 0;
  assert.equal(await module.onUtilityClick("support", () => { clicked += 1; }), true);
  assert.equal(await module.openUtility({ utilityId: "support" }), true);
  assert.equal(clicked, 1);

  const info = await module.getInfo("support");
  assert.equal(info.id, "support");
  assert.equal(info.label, "Cases");
  assert.equal(info.icon, "utility:case");
  assert.equal(info.highlighted, true);
  assert.equal(info.panelVisible, true);

  assert.equal(await module.minimize("support"), true);
  assert.equal((await module.getUtilityInfo({ utilityId: "support" })).panelVisible, false);
  assert.equal(await module.closeUtility({ utilityId: "support" }), true);
  assert.equal((await module.getAllUtilityInfo()).length, 1);
});

test("uiLearningPlatformApi returns hosted-unavailable errors with Glade code", async () => {
  installBrowserContext();
  const module = await import("../src/lightning/uiLearningPlatformApi.mjs");

  assert.equal(typeof module.getLearningItem, "function");
  assert.equal(typeof module.getLearningItems, "function");
  assert.equal(typeof module.createUnavailableError, "function");
  await assert.rejects(
    module.getLearningItem({ learningItemId: "0LI000000000001" }),
    (err) => err?.body?.errorCode === "GLADELWC095",
  );
  assert.equal(window.__gladeDiagnostics.at(-1).code, "GLADELWC095");
});

test("experience APIs expose deterministic local CMS and hosted authoring envelopes", async () => {
  installBrowserContext({
    community: {
      siteId: "0DM000000000001",
      networkId: "0DB000000000001",
      managedContent: {
        welcome: {
          id: "20Y000000000001",
          contentKey: "welcome",
          title: "Welcome",
          urlName: "welcome",
          contentBody: { headline: "Hello" },
        },
      },
    },
  });

  const delivery = await import("../src/experience/cmsDeliveryApi.mjs");
  assert.deepEqual(await delivery.getContent({ contentKeyOrId: "welcome" }), {
    contentKey: "welcome",
    contentType: "managedContent",
    id: "20Y000000000001",
    managedContentId: "20Y000000000001",
    title: "Welcome",
    urlName: "welcome",
    contentBody: { headline: "Hello" },
  });
  assert.deepEqual(await delivery.getContents({ contentKeys: ["welcome"] }), {
    currentPage: 0,
    items: [{
      contentKey: "welcome",
      contentType: "managedContent",
      id: "20Y000000000001",
      managedContentId: "20Y000000000001",
      title: "Welcome",
      urlName: "welcome",
      contentBody: { headline: "Hello" },
    }],
    pageSize: 25,
    total: 1,
    totalPages: 1,
  });

  const editor = await import("../src/experience/cmsEditorApi.mjs");
  assert.equal((await editor.getContext()).contentKey, "welcome");
  assert.equal((await editor.getContent()).title, "Welcome");
  await assert.rejects(
    editor.updateContent({ title: "Draft" }),
    (err) => err?.body?.errorCode === "GLADELWC094",
  );

  const blockBuilder = await import("../src/experience/blockBuilderApi.mjs");
  assert.equal((await blockBuilder.getCurrentSelectedBlock()).data, undefined);
  await assert.rejects(
    blockBuilder.replaceBlock({ type: "text" }, { nodeId: "node-1" }),
    (err) => err?.body?.errorCode === "GLADELWC093",
  );
});
