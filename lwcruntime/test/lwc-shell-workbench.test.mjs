import assert from "node:assert/strict";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import { defaultSLDSHref, repoRoot, startLWCDevServer } from "./helpers.mjs";

const shellFiles = {
  "/lightning/runtime/shell/app.js": "lwcruntime/src/shell/app.mjs",
  "/lightning/runtime/shell/app.mjs": "lwcruntime/src/shell/app.mjs",
  "/lightning/runtime/shell/community-host.js": "lwcruntime/src/shell/community-host.mjs",
  "/lightning/runtime/shell/community-host.mjs": "lwcruntime/src/shell/community-host.mjs",
  "/lightning/runtime/shell/router.js": "lwcruntime/src/shell/router.mjs",
  "/lightning/runtime/shell/router.mjs": "lwcruntime/src/shell/router.mjs",
  "/lightning/runtime/shell/context-panel.js": "lwcruntime/src/shell/context-panel.mjs",
  "/lightning/runtime/shell/context-panel.mjs": "lwcruntime/src/shell/context-panel.mjs",
  "/lightning/runtime/shell/diagnostics.js": "lwcruntime/src/shell/diagnostics.mjs",
  "/lightning/runtime/shell/diagnostics.mjs": "lwcruntime/src/shell/diagnostics.mjs",
  "/lightning/runtime/shell/navigation-service.js": "lwcruntime/src/shell/navigation-service.mjs",
  "/lightning/runtime/shell/navigation-service.mjs": "lwcruntime/src/shell/navigation-service.mjs",
  "/lightning/runtime/shell/workbench-lab.js": "lwcruntime/src/shell/workbench-lab.mjs",
  "/lightning/runtime/shell/workbench-lab.mjs": "lwcruntime/src/shell/workbench-lab.mjs",
  "/lightning/runtime/shell/workbench-builder.js": "lwcruntime/src/shell/workbench-builder.mjs",
  "/lightning/runtime/shell/workbench-builder.mjs": "lwcruntime/src/shell/workbench-builder.mjs",
  "/lightning/runtime/shell/workbench-console.js": "lwcruntime/src/shell/workbench-console.mjs",
  "/lightning/runtime/shell/workbench-console.mjs": "lwcruntime/src/shell/workbench-console.mjs",
  "/lightning/runtime/shell/toast-service.js": "lwcruntime/src/shell/toast-service.mjs",
  "/lightning/runtime/shell/toast-service.mjs": "lwcruntime/src/shell/toast-service.mjs",
  "/lightning/runtime/slds/slds-loader.js": "lwcruntime/src/slds/slds-loader.mjs",
  "/lightning/runtime/slds/slds-loader.mjs": "lwcruntime/src/slds/slds-loader.mjs",
  "/lightning/runtime/shims/community.js": "lwcruntime/src/shims/community.mjs",
  "/lightning/runtime/shims/community.mjs": "lwcruntime/src/shims/community.mjs",
  "/lightning/runtime/shims/lds-cache.js": "lwcruntime/src/shims/lds-cache.mjs",
  "/lightning/runtime/shims/lds-cache.mjs": "lwcruntime/src/shims/lds-cache.mjs",
  "/lightning/runtime/shims/wire-adapter.js": "lwcruntime/src/shims/wire-adapter.mjs",
  "/lightning/runtime/shims/wire-adapter.mjs": "lwcruntime/src/shims/wire-adapter.mjs",
};

function startWorkbenchServer() {
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, "http://localhost");
    if (url.pathname === "/lwc/preview/record/Account/001000000000001AAA") {
      const config = {
        title: "Glade Lightning Shell",
        activeRoute: "/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page",
        pageReference: {
          type: "standard__recordPage",
          attributes: {
            objectApiName: "Account",
            recordId: "001000000000001AAA",
            actionName: "view",
          },
          state: { page: "Account_Record_Page" },
        },
      };
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(`<!DOCTYPE html>
<html>
<head>
  <script type="importmap">
    {
      "imports": {
        "@glade/slds": "/lightning/runtime/slds/slds-loader.js",
        "@glade/shell/diagnostics": "/lightning/runtime/shell/diagnostics.js"
      }
    }
  </script>
  <script type="application/json" id="glade-lightning-config">${JSON.stringify(config)}</script>
</head>
<body>
  <header><a data-glade-route href="/lwc">Workbench</a></header>
  <div data-glade-workbench-console data-glade-workbench-mode="recordPage">
    <main data-glade-main>Mounted component region</main>
    <section data-glade-flow-events></section>
    <aside data-glade-context-panel></aside>
    <footer data-glade-debug-dock>
      <div role="tablist" aria-label="Debug output">
        <button type="button" role="tab" id="glade-debug-tab-console" data-glade-debug-tab="console" aria-controls="glade-debug-panel-console" aria-selected="true">Console</button>
        <button type="button" role="tab" id="glade-debug-tab-apex" data-glade-debug-tab="apex" aria-controls="glade-debug-panel-apex" aria-selected="false">Apex</button>
        <button type="button" role="tab" id="glade-debug-tab-lds" data-glade-debug-tab="lds" aria-controls="glade-debug-panel-lds" aria-selected="false">LDS Cache</button>
        <button type="button" role="tab" id="glade-debug-tab-network" data-glade-debug-tab="network" aria-controls="glade-debug-panel-network" aria-selected="false">Network</button>
        <button type="button" role="tab" id="glade-debug-tab-events" data-glade-debug-tab="events" aria-controls="glade-debug-panel-events" aria-selected="false">Events</button>
        <button type="button" role="tab" id="glade-debug-tab-issues" data-glade-debug-tab="issues" aria-controls="glade-debug-panel-issues" aria-selected="false">Issues</button>
      </div>
      <section id="glade-debug-panel-console" role="tabpanel" aria-labelledby="glade-debug-tab-console" data-glade-debug-panel="console"><pre data-glade-debug-output="console"></pre></section>
      <section id="glade-debug-panel-apex" role="tabpanel" aria-labelledby="glade-debug-tab-apex" data-glade-debug-panel="apex" hidden><pre data-glade-debug-output="apex"></pre></section>
      <section id="glade-debug-panel-lds" role="tabpanel" aria-labelledby="glade-debug-tab-lds" data-glade-debug-panel="lds" hidden><pre data-glade-debug-output="lds"></pre></section>
      <section id="glade-debug-panel-network" role="tabpanel" aria-labelledby="glade-debug-tab-network" data-glade-debug-panel="network" hidden><pre data-glade-debug-output="network"></pre></section>
      <section id="glade-debug-panel-events" role="tabpanel" aria-labelledby="glade-debug-tab-events" data-glade-debug-panel="events" hidden><pre data-glade-debug-output="events"></pre></section>
      <section id="glade-debug-panel-issues" role="tabpanel" aria-labelledby="glade-debug-tab-issues" data-glade-debug-panel="issues" hidden><pre data-glade-debug-output="issues"></pre></section>
    </footer>
  </div>
  <script type="module">
    import { bootGladeShell } from "/lightning/runtime/shell/app.js";
    import { reportDiagnostic } from "/lightning/runtime/shell/diagnostics.js";
    window.__boot = await bootGladeShell();
    reportDiagnostic({ code: "GLADELWC999", message: "probe diagnostic" });
    document.querySelector("[data-glade-main]").dispatchEvent(new CustomEvent("flownavigationnext", {
      bubbles: true,
      composed: true,
      detail: { action: "NEXT" },
    }));
  </script>
</body>
</html>`);
      return;
    }
    const normalizedPath = url.pathname.replace(/\.mjs$/, ".js");
    const file = shellFiles[normalizedPath] || runtimeSLDSFile(url.pathname);
    if (!file) {
      res.writeHead(404);
      res.end("missing " + url.pathname);
      return;
    }
    const filePath = path.join(repoRoot, file);
    const contentType = url.pathname.endsWith(".css")
      ? "text/css; charset=utf-8"
      : "application/javascript; charset=utf-8";
    res.writeHead(200, { "Content-Type": contentType });
    res.end(fs.readFileSync(filePath));
  });
  return new Promise((resolve) => {
    server.listen(0, "127.0.0.1", () => {
      const { port } = server.address();
      resolve({
        baseURL: `http://127.0.0.1:${port}`,
        close: () => new Promise((r) => server.close(r)),
      });
    });
  });
}

function runtimeSLDSFile(pathname) {
  if (!pathname.startsWith("/lightning/runtime/slds/")) {
    return "";
  }
  return path.join("lwcruntime/src/slds", pathname.slice("/lightning/runtime/slds/".length));
}

async function expandWorkbenchConsole(page) {
  await page.locator("[data-glade-debug-restore]").waitFor({ state: "attached", timeout: 60000 });
  const minimized = await page.locator("[data-glade-debug-dock]").evaluate((dock) => dock.dataset.gladeDebugMinimized === "true");
  if (!minimized) {
    return;
  }
  await page.locator("[data-glade-debug-restore]").click();
  await page.waitForFunction(() => document.querySelector("[data-glade-debug-dock]")?.dataset.gladeDebugMinimized !== "true", null, { timeout: 60000 });
}

test("Component Lab renders an empty component list without existing options", async () => {
  const server = await startWorkbenchServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA`, { waitUntil: "domcontentloaded" });
    await page.setContent(`<!DOCTYPE html>
<html>
<body>
  <script type="application/json" id="glade-lwc-workbench">${JSON.stringify({
    components: [],
    sampleRecordId: "001000000000001AAA",
    active: { context: {} },
  })}</script>
  <div data-glade-workbench-console>
    <section data-glade-component-lab>
      <aside data-glade-lab-components-rail>
        <input type="search" data-glade-lab-component-search>
        <div data-glade-lab-component-list></div>
      </aside>
      <section class="glade-lab-preview">
        <header>
          <code data-glade-lab-selected></code>
          <button type="button" data-glade-lab-view-option="setup"></button>
          <button type="button" data-glade-lab-view-option="preview"></button>
          <button type="button" data-glade-lab-form-factor-option="Large"></button>
          <button type="button" data-glade-lab-fit-option="fit"></button>
          <a data-glade-lab-route-link href="/lwc"></a>
        </header>
        <div data-glade-lab-context-strip>
          <button type="button" data-glade-lab-strip-part="component"></button>
          <button type="button" data-glade-lab-strip-part="context"></button>
          <button type="button" data-glade-lab-strip-part="object"></button>
          <button type="button" data-glade-lab-strip-part="record"></button>
          <button type="button" data-glade-lab-strip-part="formFactor"></button>
          <button type="button" data-glade-lab-strip-part="state"></button>
        </div>
        <div data-glade-lab-host-shell><div id="glade-component-lab-host" data-glade-lab-host></div></div>
      </section>
      <aside data-glade-lab-props-rail>
        <section data-glade-lab-context></section>
        <button type="button" data-glade-lab-reset-props></button>
        <div data-glade-lab-prop-list></div>
      </aside>
    </section>
  </div>
</body>
</html>`);
    await page.addScriptTag({
      type: "module",
      content: `import { bootComponentLab } from "/lightning/runtime/shell/workbench-lab.js"; window.__lab = bootComponentLab(document.body);`,
    });

    await page.locator("[data-glade-lab-component-empty]").waitFor({ timeout: 60000 });
    assert.equal(await page.locator("[data-glade-lab-component-empty]").isVisible(), true);
    assert.match(await page.locator("[data-glade-lab-component-empty]").innerText(), /No exposed components found/);
    assert.match(await page.locator("[data-glade-lab-host]").innerText(), /No component selected/);
  } finally {
    await browser.close();
    await server.close();
  }
});

test("Workbench Console collects runtime events and switches debug tabs with aria-selected", async () => {
  const server = await startWorkbenchServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`, { waitUntil: "networkidle" });
    await page.locator('[data-glade-debug-output="issues"]').waitFor({ state: "attached", timeout: 60000 });
    await expandWorkbenchConsole(page);

    await page.evaluate(() => {
      console.log("console probe ready", { source: "workbench" });
      document.dispatchEvent(new CustomEvent("glade:runtime-event", {
        detail: {
          kind: "apex",
          label: "AccountController.load",
          status: "success",
          detail: { className: "AccountController", method: "load" },
        },
      }));
      document.dispatchEvent(new CustomEvent("glade:runtime-event", {
        detail: {
          kind: "lds",
          label: "notifyRecordUpdateAvailable",
          detail: { recordIds: ["001000000000001AAA"] },
        },
      }));
      document.dispatchEvent(new CustomEvent("glade:runtime-event", {
        detail: {
          kind: "network",
          label: "/lightning/wire/getRecord",
          status: "success",
          detail: { endpoint: "/lightning/wire/getRecord" },
        },
      }));
    });
    await page.evaluate(async () => {
      const { createFetchWireAdapter, invokeApex } = await import("/lightning/runtime/shims/wire-adapter.js");
      const { notifyRecordUpdateAvailable } = await import("/lightning/runtime/shims/lds-cache.js");
      const { emitPageReference } = await import("/lightning/runtime/shell/navigation-service.js");
      const originalFetch = window.fetch;
      window.fetch = async (endpoint) => ({
        ok: true,
        status: 200,
        json: async () => {
          if (endpoint === "/lightning/wire/apex") {
            return { data: { loaded: true } };
          }
          return { data: { endpoint, fields: { Id: "001000000000777AAA" } } };
        },
      });
      try {
        const Adapter = createFetchWireAdapter("/lightning/wire/taskFiveProbe", () => ({
          recordId: "001000000000777AAA",
        }));
        const adapter = new Adapter(() => {});
        await adapter.update({ recordId: "001000000000777AAA" });
        await notifyRecordUpdateAvailable([{ recordId: "001000000000777AAA" }]);
        await invokeApex("TaskFiveController", "refresh", { recordId: "001000000000777AAA" });
        emitPageReference({
          type: "standard__recordPage",
          attributes: {
            objectApiName: "Account",
            recordId: "001000000000777AAA",
            actionName: "view",
          },
          state: { page: "Task_Five_Page" },
        });
      } finally {
        window.fetch = originalFetch;
      }
    });

    async function activeDebugState() {
      return page.evaluate(() => ({
        tabs: Array.from(document.querySelectorAll("[data-glade-debug-tab]")).map((tab) => ({
          kind: tab.dataset.gladeDebugTab,
          selected: tab.getAttribute("aria-selected"),
          pressed: tab.getAttribute("aria-pressed"),
        })),
        panels: Array.from(document.querySelectorAll("[data-glade-debug-panel]")).map((panel) => ({
          kind: panel.dataset.gladeDebugPanel,
          hidden: panel.hidden,
        })),
      }));
    }

    const initialState = await activeDebugState();
    assert.equal(initialState.tabs.find((tab) => tab.kind === "console").selected, "true");
    assert.equal(initialState.tabs.find((tab) => tab.kind === "apex").selected, "false");
    assert.equal(initialState.tabs.every((tab) => tab.pressed === null), true);
    assert.equal(initialState.panels.find((panel) => panel.kind === "console").hidden, false);
    assert.equal(initialState.panels.find((panel) => panel.kind === "apex").hidden, true);
    assert.match(await page.locator('[data-glade-debug-output="console"]').innerText(), /console probe ready/);
    assert.match(await page.locator('[data-glade-debug-output="console"]').innerText(), /workbench/);

    await page.locator('[data-glade-debug-tab="apex"]').click();
    assert.equal(await page.locator('[data-glade-debug-tab="apex"]').getAttribute("aria-selected"), "true");
    assert.equal(await page.locator('[data-glade-debug-panel="apex"]').evaluate((node) => node.hidden), false);
    assert.equal(await page.locator('[data-glade-debug-panel="console"]').evaluate((node) => node.hidden), true);
    assert.match(await page.locator('[data-glade-debug-output="apex"]').innerText(), /AccountController\.load/);
    assert.match(await page.locator('[data-glade-debug-output="apex"]').innerText(), /TaskFiveController\.refresh/);

    await page.locator('[data-glade-debug-tab="lds"]').click();
    assert.equal(await page.locator('[data-glade-debug-tab="lds"]').getAttribute("aria-selected"), "true");
    assert.equal(await page.locator('[data-glade-debug-panel="lds"]').evaluate((node) => node.hidden), false);
    assert.equal(await page.locator('[data-glade-debug-panel="apex"]').evaluate((node) => node.hidden), true);
    assert.match(await page.locator('[data-glade-debug-output="lds"]').innerText(), /notifyRecordUpdateAvailable/);
    assert.match(await page.locator('[data-glade-debug-output="lds"]').innerText(), /writeLDSCache/);

    await page.locator('[data-glade-debug-tab="network"]').click();
    assert.equal(await page.locator('[data-glade-debug-tab="network"]').getAttribute("aria-selected"), "true");
    assert.match(await page.locator('[data-glade-debug-output="network"]').innerText(), /\/lightning\/wire\/getRecord/);
    assert.match(await page.locator('[data-glade-debug-output="network"]').innerText(), /\/lightning\/wire\/taskFiveProbe/);
    assert.match(await page.locator('[data-glade-debug-output="network"]').innerText(), /\/lightning\/wire\/apex/);

    await page.locator('[data-glade-debug-tab="events"]').click();
    const eventsText = await page.locator('[data-glade-debug-output="events"]').innerText();
    assert.match(eventsText, /AccountController\.load/);
    assert.match(eventsText, /notifyRecordUpdateAvailable/);
    assert.match(eventsText, /\/lightning\/wire\/getRecord/);
    assert.match(eventsText, /PageReference/);

    await page.locator('[data-glade-debug-tab="issues"]').click();
    assert.match(await page.locator('[data-glade-debug-output="issues"]').innerText(), /GLADELWC999/);
    assert.match(await page.locator('[data-glade-debug-output="issues"]').innerText(), /probe diagnostic/);
	  } finally {
	    await browser.close();
	    await server.close();
	  }
	});

test("Workbench Console groups save runs and restores the latest dev-loop signal", async () => {
  const server = await startWorkbenchServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`, { waitUntil: "networkidle" });
    await page.locator("[data-glade-run-summary]").waitFor({ state: "attached", timeout: 60000 });
    await expandWorkbenchConsole(page);

    await page.evaluate(() => {
      const changedFiles = [
        "force-app/main/default/lwc/accountWorkspace/accountWorkspace.js",
        "force-app/main/default/classes/PreviewWorkflowController.cls",
      ];
      document.dispatchEvent(new CustomEvent("glade:dev-run", {
        detail: {
          id: "save-001",
          sequence: 1,
          status: "running",
          label: "Saved 2 files",
          changedFiles,
          startedAt: "2026-06-29T12:00:00Z",
        },
      }));
      console.log("selected Account", { id: "001000000000001AAA" });
      document.dispatchEvent(new CustomEvent("glade:runtime-event", {
        detail: {
          kind: "apex",
          label: "PreviewWorkflowController.findAccounts",
          status: "success",
          detail: {
            className: "PreviewWorkflowController",
            method: "findAccounts",
            params: { searchKey: "Local" },
            result: [{ Id: "001000000000001AAA", Name: "Local Preview Account" }],
            durationMs: 34,
          },
        },
      }));
      document.dispatchEvent(new CustomEvent("glade:runtime-event", {
        detail: {
          kind: "lds",
          label: "getRecord",
          status: "success",
          detail: {
            endpoint: "/lightning/wire/getRecord",
            body: { recordId: "001000000000001AAA", fields: ["Account.Name"] },
            result: { fields: { Name: { value: "Local Preview Account" } } },
            durationMs: 12,
          },
        },
      }));
      document.dispatchEvent(new CustomEvent("glade:dev-run", {
        detail: {
          id: "save-001",
          sequence: 2,
          status: "success",
          label: "Saved 2 files",
          changedFiles,
          durationMs: 840,
          finishedAt: "2026-06-29T12:00:00.840Z",
          reload: false,
        },
      }));
    });

    await page.locator("[data-glade-run-summary]").waitFor({ timeout: 60000 });
    const summaryText = await page.locator("[data-glade-run-summary]").innerText();
    assert.match(summaryText, /PASS/);
    assert.match(summaryText, /Saved 2 files/);
    assert.match(summaryText, /840ms/);
    assert.match(summaryText, /0 errors/);
    assert.match(summaryText, /accountWorkspace\.js/);
    assert.match(await page.locator("[data-glade-run-timeline]").innerText(), /save-001/);

    await page.locator('[data-glade-debug-tab="apex"]').click();
    const apexText = await page.locator('[data-glade-debug-output="apex"]').innerText();
    assert.match(apexText, /PreviewWorkflowController\.findAccounts/);
    assert.match(apexText, /searchKey/);
    assert.match(apexText, /Local Preview Account/);
    assert.match(apexText, /34ms/);

    await page.locator('[data-glade-debug-tab="lds"]').click();
    const ldsText = await page.locator('[data-glade-debug-output="lds"]').innerText();
    assert.match(ldsText, /getRecord/);
    assert.match(ldsText, /Account\.Name/);
    assert.match(ldsText, /Local Preview Account/);
    assert.match(ldsText, /12ms/);

    await page.locator('[data-glade-debug-tab="events"]').click();
    const eventsText = await page.locator('[data-glade-debug-output="events"]').innerText();
    assert.match(eventsText, /save-001/);
    assert.match(eventsText, /selected Account/);
    assert.match(eventsText, /PreviewWorkflowController\.findAccounts/);
    assert.match(eventsText, /getRecord/);

    await page.locator('[data-glade-debug-tab="apex"]').click();
    await page.reload({ waitUntil: "networkidle" });
    await page.locator("[data-glade-run-summary]").waitFor({ state: "attached", timeout: 60000 });
    await expandWorkbenchConsole(page);
    assert.equal(await page.locator('[data-glade-debug-tab="apex"]').getAttribute("aria-selected"), "true");
    assert.match(await page.locator("[data-glade-run-summary]").innerText(), /PASS/);
    assert.match(await page.locator('[data-glade-debug-output="apex"]').innerText(), /PreviewWorkflowController\.findAccounts/);
  } finally {
    await browser.close();
    await server.close();
  }
});

test("Workbench Console filters highlights and clears debug output", async () => {
  const server = await startWorkbenchServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`, { waitUntil: "networkidle" });
    await page.locator("[data-glade-debug-filter]").waitFor({ state: "attached", timeout: 60000 });
    await expandWorkbenchConsole(page);

    await page.evaluate(() => {
      console.log("alpha account loaded", { recordId: "001000000000001AAA" });
      console.error("boom account failed", { recordId: "001000000000001AAA" });
      document.dispatchEvent(new CustomEvent("glade:runtime-event", {
        detail: {
          kind: "apex",
          label: "AccountController.load",
          status: "success",
          detail: { className: "AccountController", method: "load", params: { recordId: "001000000000001AAA" } },
        },
      }));
      document.dispatchEvent(new CustomEvent("glade:runtime-event", {
        detail: {
          kind: "apex",
          label: "AccountController.save",
          status: "error",
          detail: { className: "AccountController", method: "save", error: "Validation failed" },
        },
      }));
    });

    await page.locator("[data-glade-debug-filter]").fill("boom");
    await page.waitForFunction(() => document.querySelector('[data-glade-debug-output="console"]')?.textContent.includes("boom account failed"), null, { timeout: 60000 });
    assert.match(await page.locator('[data-glade-debug-output="console"]').innerText(), /boom account failed/);
    assert.doesNotMatch(await page.locator('[data-glade-debug-output="console"]').innerText(), /alpha account loaded/);
    assert.ok(await page.locator('[data-glade-debug-output="console"] mark[data-glade-debug-match]').count() >= 1);

    await page.locator("[data-glade-debug-filter]").fill("");
    await page.locator("[data-glade-debug-problems]").click();
    assert.equal(await page.locator("[data-glade-debug-problems]").getAttribute("aria-pressed"), "true");
    assert.match(await page.locator('[data-glade-debug-output="console"]').innerText(), /boom account failed/);
    assert.doesNotMatch(await page.locator('[data-glade-debug-output="console"]').innerText(), /alpha account loaded/);

    await page.locator('[data-glade-debug-tab="apex"]').click();
    await page.locator("[data-glade-debug-filter]").fill("AccountController");
    assert.match(await page.locator('[data-glade-debug-output="apex"]').innerText(), /AccountController\.save/);
    assert.doesNotMatch(await page.locator('[data-glade-debug-output="apex"]').innerText(), /AccountController\.load/);
    assert.ok(await page.locator('[data-glade-debug-output="apex"] mark[data-glade-debug-match]').count() >= 1);

    await page.locator("[data-glade-debug-clear-current]").click();
    assert.match(await page.locator('[data-glade-debug-output="apex"]').innerText(), /No events yet/);

    await page.locator('[data-glade-debug-tab="events"]').click();
    assert.match(await page.locator('[data-glade-debug-output="events"]').innerText(), /AccountController\.save/);
    await page.locator("[data-glade-debug-clear-all]").click();
    assert.match(await page.locator('[data-glade-debug-output="events"]').innerText(), /No events yet/);
    assert.equal((await page.locator("[data-glade-run-summary]").innerText()).trim(), "");
  } finally {
    await browser.close();
    await server.close();
  }
});

test("Workbench Console syntax highlights debug output across tabs", async () => {
  const server = await startWorkbenchServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`, { waitUntil: "networkidle" });
    await page.locator("[data-glade-debug-filter]").waitFor({ state: "attached", timeout: 60000 });
    await expandWorkbenchConsole(page);

    await page.evaluate(() => {
      console.log("syntax console probe", { ok: true, count: 2 });
      document.dispatchEvent(new CustomEvent("glade:runtime-event", {
        detail: {
          kind: "apex",
          status: "success",
          detail: {
            className: "SyntaxController",
            method: "load",
            params: { recordId: "001000000000001AAA", limit: 2 },
            result: [{ Name: "Acme", Active__c: true }],
            durationMs: 42,
          },
        },
      }));
      document.dispatchEvent(new CustomEvent("glade:runtime-event", {
        detail: {
          kind: "lds",
          label: "getRecord",
          status: "success",
          detail: {
            endpoint: "/lightning/wire/getRecord",
            body: { recordId: "001000000000001AAA", fields: ["Account.Name"] },
            result: { fields: { Name: { value: "Acme" } } },
            durationMs: 12,
          },
        },
      }));
      document.dispatchEvent(new CustomEvent("glade:runtime-event", {
        detail: {
          kind: "network",
          label: "/lightning/wire/apex",
          status: "error",
          detail: { endpoint: "/lightning/wire/apex", status: 500, body: { message: "Apex failed" } },
        },
      }));
      document.dispatchEvent(new CustomEvent("glade:diagnostic", {
        detail: { code: "GLADELWC777", message: "syntax diagnostic", severity: "error" },
      }));
    });
    async function renderedTokenCounts(kind, waitForText, tokens) {
      await page.locator(`[data-glade-debug-tab="${kind}"]`).click();
      await page.waitForFunction(({ outputKind, text }) =>
        document.querySelector(`[data-glade-debug-output="${outputKind}"]`)?.textContent.includes(text),
      { outputKind: kind, text: waitForText }, { timeout: 60000 });
      return page.evaluate(({ outputKind, tokenNames }) => {
        const count = (token) => document.querySelectorAll(`[data-glade-debug-output="${outputKind}"] [data-glade-token="${token}"]`).length;
        return Object.fromEntries(tokenNames.map((token) => [token, count(token)]));
      }, { outputKind: kind, tokenNames: tokens });
    }

    const consoleCounts = await renderedTokenCounts("console", "syntax console probe", ["status", "json-key", "json-boolean"]);
    const apexCounts = await renderedTokenCounts("apex", "SyntaxController.load", ["method", "json-key", "json-string", "json-number", "duration"]);
    const ldsCounts = await renderedTokenCounts("lds", "getRecord", ["endpoint", "json-key"]);
    const networkCounts = await renderedTokenCounts("network", "/lightning/wire/apex", ["endpoint", "status"]);
    const eventsCounts = await renderedTokenCounts("events", "SyntaxController.load", ["method", "json-key"]);
    const issuesCounts = await renderedTokenCounts("issues", "syntax diagnostic", ["status", "json-string"]);
    const tokenCounts = {
      consoleStatus: consoleCounts.status,
      consoleJsonKey: consoleCounts["json-key"],
      consoleBoolean: consoleCounts["json-boolean"],
      apexMethod: apexCounts.method,
      apexJsonKey: apexCounts["json-key"],
      apexString: apexCounts["json-string"],
      apexNumber: apexCounts["json-number"],
      apexDuration: apexCounts.duration,
      ldsEndpoint: ldsCounts.endpoint,
      ldsJsonKey: ldsCounts["json-key"],
      networkEndpoint: networkCounts.endpoint,
      networkErrorStatus: networkCounts.status,
      eventsMethod: eventsCounts.method,
      eventsJsonKey: eventsCounts["json-key"],
      issuesStatus: issuesCounts.status,
      issuesJsonString: issuesCounts["json-string"],
    };
    for (const [name, value] of Object.entries(tokenCounts)) {
      assert.ok(value > 0, `${name} = ${value}`);
    }
  } finally {
    await browser.close();
    await server.close();
  }
});

test("Workbench Console lazily renders debug tabs to keep heavy output off the scroll path", async () => {
  const server = await startWorkbenchServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.addInitScript(() => sessionStorage.clear());
    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`, { waitUntil: "networkidle" });
    await page.locator("[data-glade-debug-filter]").waitFor({ state: "attached", timeout: 60000 });
    await expandWorkbenchConsole(page);

    await page.evaluate(() => {
      for (let index = 0; index < 120; index += 1) {
        console.log("perf console event", { index, ok: true, payload: "x".repeat(80) });
        document.dispatchEvent(new CustomEvent("glade:runtime-event", {
          detail: {
            kind: "apex",
            status: "success",
            detail: {
              className: "PerfController",
              method: "load",
              params: { index, searchKey: "Local" },
              result: [{ Id: "001000000000001AAA", Name: "Local Preview Account", Count__c: index }],
              durationMs: 9,
            },
          },
        }));
        document.dispatchEvent(new CustomEvent("glade:runtime-event", {
          detail: {
            kind: "lds",
            label: "getRecord",
            status: "success",
            detail: {
              endpoint: "/lightning/wire/getRecord",
              body: { recordId: "001000000000001AAA", fields: ["Account.Name", "Account.Industry"] },
              result: { fields: { Name: { value: "Local Preview Account" }, Industry: { value: "Technology" } } },
              durationMs: 4,
            },
          },
        }));
        document.dispatchEvent(new CustomEvent("glade:runtime-event", {
          detail: {
            kind: "network",
            label: "/lightning/wire/apex",
            status: "success",
            detail: { endpoint: "/lightning/wire/apex", status: 200, body: { index, ok: true } },
          },
        }));
      }
    });
    await page.waitForFunction(() => document.querySelector('[data-glade-debug-output="console"]')?.textContent.includes("perf console event"), null, { timeout: 60000 });

    const consoleTabCounts = await page.evaluate(() => Object.fromEntries(
      Array.from(document.querySelectorAll("[data-glade-debug-output]")).map((output) => [
        output.dataset.gladeDebugOutput,
        output.querySelectorAll("*").length,
      ]),
    ));
    assert.ok(consoleTabCounts.console > 0, `console node count = ${consoleTabCounts.console}`);
    for (const kind of ["apex", "lds", "network", "events", "issues"]) {
      assert.equal(consoleTabCounts[kind], 0, `${kind} should stay empty until selected`);
    }
    assert.ok(await page.locator("[data-glade-debug-dock] *").count() < 4000);

    await page.locator('[data-glade-debug-tab="apex"]').click();
    await page.waitForFunction(() => document.querySelector('[data-glade-debug-output="apex"]')?.textContent.includes("PerfController.load"), null, { timeout: 60000 });
    const apexTabCounts = await page.evaluate(() => Object.fromEntries(
      Array.from(document.querySelectorAll("[data-glade-debug-output]")).map((output) => [
        output.dataset.gladeDebugOutput,
        output.querySelectorAll("*").length,
      ]),
    ));
    assert.ok(apexTabCounts.apex > 0, `apex node count = ${apexTabCounts.apex}`);
    assert.equal(apexTabCounts.console, 0);
    assert.equal(apexTabCounts.lds, 0);
    assert.equal(apexTabCounts.network, 0);
    assert.equal(apexTabCounts.events, 0);
  } finally {
    await browser.close();
    await server.close();
  }
});

test("Workbench Console batches persistence during runtime event bursts", async () => {
  const server = await startWorkbenchServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.addInitScript(() => sessionStorage.clear());
    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`, { waitUntil: "networkidle" });
    await page.locator("[data-glade-debug-filter]").waitFor({ state: "attached", timeout: 60000 });
    await expandWorkbenchConsole(page);

    const result = await page.evaluate(async () => {
      const originalSetItem = Storage.prototype.setItem;
      let writeCount = 0;
      Storage.prototype.setItem = function setItemProbe(key, ...args) {
        if (key === "glade:workbench-console:v2") {
          writeCount += 1;
        }
        return originalSetItem.call(this, key, ...args);
      };
      try {
        for (let index = 0; index < 80; index += 1) {
          console.log("persistence burst", { index, ok: true });
          document.dispatchEvent(new CustomEvent("glade:runtime-event", {
            detail: {
              kind: "apex",
              status: "success",
              detail: {
                className: "PersistenceController",
                method: "load",
                params: { index },
                result: [{ Name: "Local Preview Account", Index__c: index }],
              },
            },
          }));
        }
        await Promise.resolve();
        return {
          writeCount,
          consoleEvents: window.__gladeWorkbenchEvents?.console?.length || 0,
          apexEvents: window.__gladeWorkbenchEvents?.apex?.length || 0,
        };
      } finally {
        Storage.prototype.setItem = originalSetItem;
      }
    });

    assert.equal(result.consoleEvents, 80);
    assert.equal(result.apexEvents, 80);
    assert.ok(result.writeCount <= 3, `sessionStorage writes = ${result.writeCount}`);
  } finally {
    await browser.close();
    await server.close();
  }
});

test("Workbench Console keeps long debug output scrollable", async (t) => {
  const server = await startLWCDevServer(t, {
    projectRel: "testdata/local-tests/lwc-shell",
    pagePath: "/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page",
  });
  if (!server) {
    return;
  }
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1180, height: 620 } });
    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`, { waitUntil: "networkidle" });
    await page.locator("[data-glade-debug-filter]").waitFor({ state: "attached", timeout: 60000 });
    await expandWorkbenchConsole(page);

	    await page.evaluate(() => {
	      for (let i = 0; i < 90; i += 1) {
	        console.log(`scroll probe ${String(i).padStart(2, "0")}`, { index: i });
	        document.dispatchEvent(new CustomEvent("glade:runtime-event", {
	          detail: {
	            kind: "lds",
	            label: "writeLDSCache",
	            status: "write",
	            detail: {
	              key: `/lightning/wire/getRecord/${i}`,
	              body: { recordId: `001000000000${String(i).padStart(3, "0")}AAA`, fields: ["Account.Name"] },
	            },
	          },
	        }));
	      }
	    });
	    await page.waitForFunction(() => document.querySelector('[data-glade-debug-output="console"]')?.textContent.includes("scroll probe 89"), null, { timeout: 60000 });

    const metrics = await page.locator('[data-glade-debug-output="console"]').evaluate((node) => {
      node.scrollTop = node.scrollHeight;
      return {
        clientHeight: node.clientHeight,
        scrollHeight: node.scrollHeight,
        scrollTop: node.scrollTop,
        overflowY: getComputedStyle(node).overflowY,
        panelOverflowY: getComputedStyle(node.closest("[role='tabpanel']")).overflowY,
      };
    });
    assert.match(metrics.overflowY, /auto|scroll/);
    assert.equal(metrics.panelOverflowY, "hidden");
	    assert.ok(metrics.clientHeight > 0, `client height = ${metrics.clientHeight}`);
	    assert.ok(metrics.scrollHeight > metrics.clientHeight + 20, `scroll ${metrics.scrollHeight}, client ${metrics.clientHeight}`);
	    assert.ok(metrics.scrollTop > 0, `scrollTop = ${metrics.scrollTop}`);

	    async function assertPaneUsesDockHeight(kind, waitForText) {
	      await page.locator(`[data-glade-debug-tab="${kind}"]`).click();
	      await page.waitForFunction(({ outputKind, text }) =>
	        document.querySelector(`[data-glade-debug-output="${outputKind}"]`)?.textContent.includes(text),
	      { outputKind: kind, text: waitForText }, { timeout: 60000 });
	      return page.evaluate((outputKind) => {
	        const panel = document.querySelector(`[data-glade-debug-panel="${outputKind}"]`);
	        const output = document.querySelector(`[data-glade-debug-output="${outputKind}"]`);
	        const panelBox = panel.getBoundingClientRect();
	        const outputBox = output.getBoundingClientRect();
	        const outputStyle = getComputedStyle(output);
	        output.scrollTop = output.scrollHeight;
	        output.scrollLeft = output.scrollWidth;
	        return {
	          panelHeight: panelBox.height,
	          outputHeight: outputBox.height,
	          outputOverflowX: outputStyle.overflowX,
	          outputOverflowY: outputStyle.overflowY,
	          outputScrollbarColor: outputStyle.scrollbarColor,
	          outputScrollTop: output.scrollTop,
	          outputScrollLeft: output.scrollLeft,
	        };
	      }, kind);
	    }
	    const ldsPane = await assertPaneUsesDockHeight("lds", "writeLDSCache");
	    const eventsPane = await assertPaneUsesDockHeight("events", "writeLDSCache");
	    for (const pane of [ldsPane, eventsPane]) {
	      assert.ok(pane.outputHeight >= pane.panelHeight - 2, `output ${pane.outputHeight}, panel ${pane.panelHeight}`);
	      assert.match(pane.outputOverflowX, /auto|scroll/);
	      assert.match(pane.outputOverflowY, /auto|scroll/);
	      assert.notEqual(pane.outputScrollbarColor, "auto");
	      assert.ok(pane.outputScrollTop > 0, `scrollTop = ${pane.outputScrollTop}`);
	      assert.ok(pane.outputScrollLeft >= 0, `scrollLeft = ${pane.outputScrollLeft}`);
	    }
	  } finally {
	    await browser.close();
	    await server.close();
	  }
	});

test("Workbench Console resizes vertically from the drag handle", async (t) => {
  const server = await startLWCDevServer(t, {
    projectRel: "testdata/local-tests/lwc-shell",
    pagePath: "/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page",
  });
  if (!server) {
    return;
  }
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 760 } });
    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`, { waitUntil: "networkidle" });
    await page.locator("[data-glade-debug-restore]").waitFor({ state: "attached", timeout: 60000 });

    const defaultMinimized = await page.evaluate(() => {
      const dock = document.querySelector("[data-glade-debug-dock]");
      const restore = document.querySelector("[data-glade-debug-restore]");
      const summary = document.querySelector("[data-glade-run-summary]");
      return {
        minimized: dock?.dataset.gladeDebugMinimized || "",
        dockHeight: dock?.getBoundingClientRect().height || 0,
        restoreVisible: (restore?.getBoundingClientRect().width || 0) > 0,
        restoreText: restore?.textContent?.trim() || "",
        restoreAria: restore?.getAttribute("aria-label") || "",
        summaryText: summary?.textContent?.trim() || "",
        toolsVisible: (document.querySelector("[data-glade-debug-tools]")?.getBoundingClientRect().height || 0) > 0,
        documentVerticalOverflow: document.documentElement.scrollHeight > document.documentElement.clientHeight + 1,
      };
    });
    assert.equal(defaultMinimized.minimized, "true");
    assert.equal(defaultMinimized.restoreVisible, true);
    assert.equal(defaultMinimized.restoreText, "↑");
    assert.equal(defaultMinimized.restoreAria, "Expand console");
    assert.equal(defaultMinimized.summaryText, "");
    assert.equal(defaultMinimized.toolsVisible, false);
    assert.equal(defaultMinimized.documentVerticalOverflow, false);
    assert.ok(defaultMinimized.dockHeight <= 56, `default dock height = ${defaultMinimized.dockHeight}`);

    await expandWorkbenchConsole(page);
    await page.locator("[data-glade-debug-resize-handle]").waitFor({ state: "visible", timeout: 60000 });

    const expandedChrome = await page.evaluate(() => {
      const dock = document.querySelector("[data-glade-debug-dock]")?.getBoundingClientRect();
      const resizeShell = document.querySelector("[data-glade-debug-resize-shell]")?.getBoundingClientRect();
      const tools = document.querySelector("[data-glade-debug-tools]")?.getBoundingClientRect();
      return {
        resizeShellHeight: resizeShell?.height || 0,
        toolsOffset: tools && dock ? tools.top - dock.top : -1,
        minimizeInTools: Boolean(document.querySelector("[data-glade-debug-tools] [data-glade-debug-minimize]")),
        restoreVisible: (document.querySelector("[data-glade-debug-restore]")?.getBoundingClientRect().width || 0) > 0,
      };
    });
    assert.ok(expandedChrome.resizeShellHeight <= 8, `resize shell height = ${expandedChrome.resizeShellHeight}`);
    assert.ok(expandedChrome.toolsOffset <= 8, `tools offset = ${expandedChrome.toolsOffset}`);
    assert.equal(expandedChrome.minimizeInTools, true);
    assert.equal(expandedChrome.restoreVisible, false);

    const initial = await page.locator("[data-glade-debug-dock]").boundingBox();
    const handle = await page.locator("[data-glade-debug-resize-handle]").boundingBox();
    assert.ok(initial, "missing debug dock box");
    assert.ok(handle, "missing resize handle box");

    await page.mouse.move(handle.x + handle.width / 2, handle.y + handle.height / 2);
    await page.mouse.down();
    await page.mouse.move(handle.x + handle.width / 2, handle.y + handle.height / 2 - 120, { steps: 8 });
    await page.mouse.up();

    await page.waitForFunction((initialHeight) => {
      const dock = document.querySelector("[data-glade-debug-dock]");
      return dock && dock.getBoundingClientRect().height > initialHeight + 70;
    }, initial.height, { timeout: 60000 });

    const resized = await page.evaluate(() => {
      const box = (selector) => document.querySelector(selector).getBoundingClientRect();
      const output = document.querySelector('[data-glade-debug-output="console"]');
      return {
        dockHeight: box("[data-glade-debug-dock]").height,
        outputHeight: output.clientHeight,
        documentVerticalOverflow: document.documentElement.scrollHeight > document.documentElement.clientHeight + 1,
        handleAriaNow: document.querySelector("[data-glade-debug-resize-handle]")?.getAttribute("aria-valuenow") || "",
      };
    });
    assert.ok(resized.dockHeight > initial.height + 70, `dock ${resized.dockHeight}, initial ${initial.height}`);
    assert.ok(resized.outputHeight > 40, `output height = ${resized.outputHeight}`);
    assert.equal(resized.documentVerticalOverflow, false);
    assert.equal(Number(resized.handleAriaNow), Math.round(resized.dockHeight));

    await page.reload({ waitUntil: "networkidle" });
    await page.locator("[data-glade-debug-resize-handle]").waitFor({ state: "attached", timeout: 60000 });
    const afterReload = await page.locator("[data-glade-debug-dock]").boundingBox();
    assert.ok(afterReload, "missing dock after reload");
    assert.ok(afterReload.height > initial.height + 70, `after reload ${afterReload.height}, initial ${initial.height}`);
    assert.ok(Math.abs(afterReload.height - resized.dockHeight) <= 2, `after reload ${afterReload.height}, resized ${resized.dockHeight}`);

    await page.locator("[data-glade-debug-minimize]").click();
    await page.waitForFunction((previousHeight) => {
      const dock = document.querySelector("[data-glade-debug-dock]");
      return dock && dock.getBoundingClientRect().height < Math.min(56, previousHeight / 2);
    }, resized.dockHeight, { timeout: 60000 });
    const minimized = await page.evaluate(() => {
      const dock = document.querySelector("[data-glade-debug-dock]");
      const output = document.querySelector('[data-glade-debug-output="console"]');
      return {
        dockHeight: dock.getBoundingClientRect().height,
        minimized: dock.dataset.gladeDebugMinimized,
        outputVisible: output.getBoundingClientRect().height > 0,
        restoreVisible: document.querySelector("[data-glade-debug-restore]")?.getBoundingClientRect().width > 0,
        documentVerticalOverflow: document.documentElement.scrollHeight > document.documentElement.clientHeight + 1,
      };
    });
    assert.equal(minimized.minimized, "true");
    assert.equal(minimized.outputVisible, false);
    assert.equal(minimized.restoreVisible, true);
    assert.equal(minimized.documentVerticalOverflow, false);
    assert.ok(minimized.dockHeight <= 56, `minimized height = ${minimized.dockHeight}`);

    await page.locator("[data-glade-debug-restore]").click();
    await page.waitForFunction((previousHeight) => {
      const dock = document.querySelector("[data-glade-debug-dock]");
      return dock && dock.getBoundingClientRect().height >= previousHeight - 2;
    }, resized.dockHeight, { timeout: 60000 });
    const restored = await page.evaluate(() => {
      const dock = document.querySelector("[data-glade-debug-dock]");
      const output = document.querySelector('[data-glade-debug-output="console"]');
      return {
        dockHeight: dock.getBoundingClientRect().height,
        minimized: dock.dataset.gladeDebugMinimized || "",
        outputVisible: output.getBoundingClientRect().height > 0,
        documentVerticalOverflow: document.documentElement.scrollHeight > document.documentElement.clientHeight + 1,
      };
    });
    assert.equal(restored.minimized, "");
    assert.equal(restored.outputVisible, true);
    assert.equal(restored.documentVerticalOverflow, false);
    assert.ok(Math.abs(restored.dockHeight - resized.dockHeight) <= 2, `restored ${restored.dockHeight}, resized ${resized.dockHeight}`);
  } finally {
    await browser.close();
    await server.close();
  }
});

test("lwc shell workbench boots context panel diagnostics toasts and route kind", async () => {
  const server = await startWorkbenchServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`, { waitUntil: "networkidle" });
    const result = await page.evaluate(() => ({
      routeKind: document.documentElement.dataset.gladeShell,
      contextText: document.querySelector("[data-glade-context-panel]")?.textContent || "",
      sldsHref: document.querySelector("link[data-glade-slds]")?.getAttribute("href") || "",
      toastRegion: Boolean(document.querySelector("[data-glade-toast-region]")),
      flowEventsText: document.querySelector("[data-glade-flow-events]")?.textContent || "",
      diagnostics: window.__gladeDiagnostics,
    }));

    assert.equal(result.routeKind, "record");
    assert.match(result.contextText, /standard__recordPage/);
    assert.match(result.contextText, /Account/);
    assert.match(result.contextText, /001000000000001AAA/);
    assert.match(result.contextText, /GLADELWC999: probe diagnostic/);
    assert.equal(result.sldsHref, defaultSLDSHref);
    assert.equal(result.toastRegion, true);
    assert.match(result.flowEventsText, /flownavigationnext/);
    assert.match(result.flowEventsText, /NEXT/);
    assert.equal(result.diagnostics.at(-1).code, "GLADELWC999");
  } finally {
    await browser.close();
    await server.close();
  }
});

test("LWC shell workbench renders routes, context, and mounted record page", async (t) => {
  const server = await startLWCDevServer(t, {
    projectRel: "testdata/local-tests/lwc-shell",
    pagePath: "/lwc",
  });
  if (!server) {
    return;
  }

  const browser = await chromium.launch({ headless: true });
  const consoleErrors = [];
  const pageErrors = [];
  try {
    const page = await browser.newPage();
    page.on("console", (msg) => {
      if (msg.type() === "error") {
        consoleErrors.push(msg.text());
      }
    });
    page.on("pageerror", (err) => {
      pageErrors.push(err.message);
    });

    await page.goto(`${server.baseURL}/lwc`, { waitUntil: "networkidle" });

    assert.equal(await page.locator("body").getAttribute("data-glade-shell"), "workbench");
    assert.equal(await page.locator("[data-glade-context-panel]").count(), 1);
    assert.equal(await page.locator("[data-glade-workbench-home]").count(), 1);
    assert.equal(await page.locator("[data-glade-workbench-builder]").count(), 0);
    assert.ok(await page.locator('[data-glade-builder-link][href="/lwc/builder"]').count());
    assert.ok(await page.locator('[data-glade-home-tab][href="/lwc/preview/tab/Lwc_Probe"]').count());
    assert.ok(await page.locator('[data-glade-home-route][href^="/lwc/preview/record/Account/"]').count());
    assert.ok(await page.locator('[data-glade-home-route][href="/lwc/preview/app/Sales_Dashboard"]').count());
    assert.ok(await page.locator('[data-glade-home-route][href="/lwc/preview/community/Partner_Portal/Account"]').count());

    const workbench = await page.locator("#glade-lwc-workbench").textContent();
    const model = JSON.parse(workbench || "{}");
    assert.equal(model.title, "Glade Lightning Shell");
    assert.ok(model.routes.some((route) => route.url === "/lwc/preview/app/Sales_Dashboard"));
    assert.ok(model.routes.some((route) => route.url === "/lwc/preview/tab/Lwc_Probe"));
    assert.ok(model.routes.some((route) => route.url === "/lwc/preview/component/c/contextProbe"));
    assert.ok(model.routes.some((route) => route.url === "/lwc/preview/community/Partner_Portal/Account"));

    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`, {
      waitUntil: "networkidle",
    });
    await expandWorkbenchConsole(page);
    await page.locator("details.glade-route-menu summary").click();
    assert.ok(await page.locator('details.glade-route-menu[open] a[data-glade-route-link][href="/lwc/preview/component/c/contextProbe"]').count());
    assert.ok(await page.locator('details.glade-route-menu[open] a[data-glade-route-link][href="/lwc/preview/tab/Lwc_Probe"]').count());
    assert.match(await page.locator("c-record-probe").innerText({ timeout: 60000 }), /Local Shell Account/);
    await assert.match(await page.locator("[data-glade-context-panel]").innerText(), /Record/);
    await assert.match(await page.locator("[data-glade-context-panel]").innerText(), /001000000000001AAA/);
    assert.equal(await page.locator("script#glade-lwc-context").evaluate((node) => JSON.parse(node.textContent).kind), "recordPage");

    const consoleMetrics = await page.evaluate(() => {
      const box = (selector) => document.querySelector(selector).getBoundingClientRect();
      const console = document.querySelector("[data-glade-workbench-console]");
      const dock = document.querySelector("[data-glade-debug-dock]");
      const viewportTolerance = 1;
      return {
        mode: console?.dataset.gladeWorkbenchMode,
        columns: getComputedStyle(console).gridTemplateColumns.split(" ").length,
        rows: getComputedStyle(console).gridTemplateRows.split(" ").length,
        documentVerticalOverflow: document.documentElement.scrollHeight > document.documentElement.clientHeight + viewportTolerance,
        sidebarWidth: box("[data-glade-workbench-sidebar]").width,
        canvasWidth: box("[data-glade-preview-canvas]").width,
        contextWidth: box("[data-glade-context-inspector]").width,
        consoleLeft: box("[data-glade-workbench-console]").left,
        consoleRight: box("[data-glade-workbench-console]").right,
        dockLeft: box("[data-glade-debug-dock]").left,
        dockRight: box("[data-glade-debug-dock]").right,
        dockHeight: box("[data-glade-debug-dock]").height,
        dockOverflow: getComputedStyle(dock).overflow,
        stageOverflowY: getComputedStyle(document.querySelector("[data-glade-preview-canvas]")).overflowY,
        sidebarOverflowY: getComputedStyle(document.querySelector("[data-glade-workbench-sidebar]")).overflowY,
        contextOverflowY: getComputedStyle(document.querySelector("[data-glade-context-inspector]")).overflowY,
        debugOutputOverflowY: getComputedStyle(document.querySelector(".glade-debug-dock pre")).overflowY,
        horizontalOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      };
    });
    assert.equal(consoleMetrics.mode, "recordPage");
    assert.equal(consoleMetrics.horizontalOverflow, false);
    assert.equal(consoleMetrics.documentVerticalOverflow, false);
    assert.equal(consoleMetrics.columns, 3);
    assert.equal(consoleMetrics.rows, 2);
    assert.ok(Math.abs(consoleMetrics.dockLeft - consoleMetrics.consoleLeft) <= 1, `dock left = ${consoleMetrics.dockLeft}, console left = ${consoleMetrics.consoleLeft}`);
    assert.ok(Math.abs(consoleMetrics.dockRight - consoleMetrics.consoleRight) <= 1, `dock right = ${consoleMetrics.dockRight}, console right = ${consoleMetrics.consoleRight}`);
    assert.equal(consoleMetrics.dockOverflow, "hidden");
    assert.match(consoleMetrics.stageOverflowY, /auto|scroll/);
    assert.match(consoleMetrics.sidebarOverflowY, /auto|scroll/);
    assert.match(consoleMetrics.contextOverflowY, /auto|scroll/);
    assert.match(consoleMetrics.debugOutputOverflowY, /auto|scroll/);
    assert.ok(consoleMetrics.sidebarWidth >= 260 && consoleMetrics.sidebarWidth <= 340);
    assert.ok(consoleMetrics.canvasWidth > consoleMetrics.sidebarWidth);
    assert.ok(consoleMetrics.canvasWidth > consoleMetrics.contextWidth);
    assert.ok(consoleMetrics.dockHeight >= 180 && consoleMetrics.dockHeight <= 300);

    await page.setViewportSize({ width: 900, height: 900 });
    const compactConsoleMetrics = await page.evaluate(() => {
      const box = (selector) => document.querySelector(selector).getBoundingClientRect();
      const console = document.querySelector("[data-glade-workbench-console]");
      const viewportTolerance = 1;
      return {
        columns: getComputedStyle(console).gridTemplateColumns.split(" ").length,
        rows: getComputedStyle(console).gridTemplateRows.split(" ").length,
        documentVerticalOverflow: document.documentElement.scrollHeight > document.documentElement.clientHeight + viewportTolerance,
        consoleLeft: box("[data-glade-workbench-console]").left,
        consoleRight: box("[data-glade-workbench-console]").right,
        dockLeft: box("[data-glade-debug-dock]").left,
        dockRight: box("[data-glade-debug-dock]").right,
        sidebarTop: box("[data-glade-workbench-sidebar]").top,
        canvasTop: box("[data-glade-preview-canvas]").top,
        contextTop: box("[data-glade-context-inspector]").top,
        dockTop: box("[data-glade-debug-dock]").top,
        sidebarHeight: box("[data-glade-workbench-sidebar]").height,
        canvasHeight: box("[data-glade-preview-canvas]").height,
        contextHeight: box("[data-glade-context-inspector]").height,
        dockHeight: box("[data-glade-debug-dock]").height,
        horizontalOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      };
    });
    assert.equal(compactConsoleMetrics.horizontalOverflow, false);
    assert.equal(compactConsoleMetrics.documentVerticalOverflow, false);
    assert.equal(compactConsoleMetrics.columns, 1);
    assert.equal(compactConsoleMetrics.rows, 4);
    assert.ok(Math.abs(compactConsoleMetrics.dockLeft - compactConsoleMetrics.consoleLeft) <= 1, `compact dock left = ${compactConsoleMetrics.dockLeft}, console left = ${compactConsoleMetrics.consoleLeft}`);
    assert.ok(Math.abs(compactConsoleMetrics.dockRight - compactConsoleMetrics.consoleRight) <= 1, `compact dock right = ${compactConsoleMetrics.dockRight}, console right = ${compactConsoleMetrics.consoleRight}`);
    assert.ok(compactConsoleMetrics.sidebarTop < compactConsoleMetrics.canvasTop);
    assert.ok(compactConsoleMetrics.canvasTop < compactConsoleMetrics.contextTop);
    assert.ok(compactConsoleMetrics.contextTop < compactConsoleMetrics.dockTop);
    assert.ok(compactConsoleMetrics.sidebarHeight <= 160, `sidebar height = ${compactConsoleMetrics.sidebarHeight}`);
    assert.ok(compactConsoleMetrics.canvasHeight >= 240, `canvas height = ${compactConsoleMetrics.canvasHeight}`);
    assert.ok(compactConsoleMetrics.contextHeight <= 180, `context height = ${compactConsoleMetrics.contextHeight}`);
    assert.ok(compactConsoleMetrics.dockHeight >= 180, `dock height = ${compactConsoleMetrics.dockHeight}`);
    await page.setViewportSize({ width: 1280, height: 720 });

    const contextResponse = await page.request.get(`${server.baseURL}/lightning/local/context.json?url=/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`);
    assert.equal(contextResponse.ok(), true);
    const context = await contextResponse.json();
    assert.equal(context.pageReference.type, "standard__recordPage");
    assert.equal(context.context.recordId, "001000000000001AAA");
    assert.ok(context.mounts.some((mount) => mount.qualified === "c:recordProbe"));

    await page.goto(`${server.baseURL}/lwc/preview/community/Partner_Portal/Account?formFactor=Large`, {
      waitUntil: "networkidle",
    });
    await page.locator("c-community-probe").waitFor({ timeout: 60000 });
    await page.locator("c-community-theme-layout").waitFor({ state: "attached", timeout: 60000 });
    assert.equal(await page.locator("c-community-theme-layout #glade-lwc-main-0").count(), 1);
    assert.match(await page.locator("c-community-theme-layout c-community-probe").innerText({ timeout: 60000 }), /Community Probe/);
    assert.match(await page.locator("c-community-probe").innerText({ timeout: 60000 }), /\/lwc\/preview\/community\/Partner_Portal\/Account\?c__view=summary/);
  } finally {
    await browser.close();
    await server.close();
  }

  assert.deepEqual(consoleErrors, []);
  assert.deepEqual(pageErrors, []);
});

test("LWC shell defaults to Component Lab and applies API props", async (t) => {
  const server = await startLWCDevServer(t, {
    projectRel: "testdata/local-tests/lwc-shell",
    pagePath: "/lwc",
  });
  if (!server) {
    return;
  }

  const browser = await chromium.launch({ headless: true });
  const consoleErrors = [];
  const pageErrors = [];
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 760 } });
    page.on("console", (msg) => {
      if (msg.type() === "error") {
        consoleErrors.push(msg.text());
      }
    });
    page.on("pageerror", (err) => {
      pageErrors.push(err.message);
    });
    await page.route("**/lightning/local/objects.json?*", async (route) => {
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({
          objects: [
            { apiName: "Account", label: "Account", recordCount: 2 },
            { apiName: "Contact", label: "Contact", recordCount: 1 },
          ],
        }),
      });
    });
    const recordSearchURLs = [];
    await page.route("**/lightning/local/records.json?*", async (route) => {
      recordSearchURLs.push(route.request().url());
      await route.fulfill({
        contentType: "application/json",
        body: JSON.stringify({
          objectApiName: "Account",
          records: [
            { id: "001000000000001AAA", title: "Local Preview Account" },
            { id: "001000000000002AAA", title: "Builder Context Account" },
          ],
        }),
      });
    });

    await page.goto(`${server.baseURL}/lwc`, { waitUntil: "networkidle" });
    await page.locator("[data-glade-component-lab]").waitFor({ timeout: 60000 });

    assert.equal(await page.locator("[data-glade-workbench-home]").count(), 1);
    assert.equal(await page.locator("[data-glade-component-lab]").count(), 1);
    assert.equal(await page.locator("[data-glade-workbench-builder]").count(), 0);
    assert.equal(await page.locator("[data-glade-lab-recent]").count(), 0);
    assert.equal(await page.locator("[data-glade-lab-component-picker]").count(), 0);
	    assert.doesNotMatch(await page.locator("[data-glade-component-lab]").innerText(), /Focus\s+Single LWC/);
	    assert.equal(await page.locator("[data-glade-lab-rail-toggle]").count(), 0);
	    assert.equal(await page.locator("[data-glade-lab-view-tabs]").count(), 1);
	    assert.equal(await page.locator('[data-glade-lab-view-option="setup"]').getAttribute("aria-pressed"), "false");
	    assert.equal(await page.locator('[data-glade-lab-view-option="preview"]').getAttribute("aria-pressed"), "true");
	    const labChrome = await page.evaluate(() => {
	      const lab = document.querySelector("[data-glade-component-lab]");
	      const viewTabs = document.querySelector("[data-glade-lab-view-tabs]");
	      const previewHeader = document.querySelector(".glade-lab-preview-header");
      return {
        directViewRows: lab ? Array.from(lab.children).filter((child) => child.matches("[data-glade-lab-view-tabs]")).length : -1,
        homeDisplay: getComputedStyle(document.querySelector("[data-glade-workbench-home]")).display,
        labDisplay: lab ? getComputedStyle(lab).display : "",
        previewDisplay: getComputedStyle(document.querySelector(".glade-lab-preview")).display,
        componentsDisplay: getComputedStyle(document.querySelector("[data-glade-lab-components-rail]")).display,
        propsDisplay: getComputedStyle(document.querySelector("[data-glade-lab-props-rail]")).display,
        gridTemplateAreas: lab ? getComputedStyle(lab).gridTemplateAreas : "",
        previewOffsetFromLabTop: lab && previewHeader ? previewHeader.closest(".glade-lab-preview").getBoundingClientRect().top - lab.getBoundingClientRect().top : -1,
        viewTabsInPreviewHeader: Boolean(viewTabs && previewHeader?.contains(viewTabs)),
      };
    });
    assert.equal(labChrome.directViewRows, 0);
	    assert.equal(labChrome.homeDisplay, "flex");
	    assert.equal(labChrome.labDisplay, "flex");
	    assert.equal(labChrome.previewDisplay, "flex");
	    assert.equal(labChrome.componentsDisplay, "none");
	    assert.equal(labChrome.propsDisplay, "none");
	    assert.doesNotMatch(labChrome.gridTemplateAreas, /viewbar/);
	    assert.ok(labChrome.previewOffsetFromLabTop <= 2, `preview offset = ${labChrome.previewOffsetFromLabTop}`);
	    assert.equal(labChrome.viewTabsInPreviewHeader, true);
    const homeChrome = await page.evaluate(() => {
      const home = document.querySelector("[data-glade-workbench-home]");
      const globalHeader = document.querySelector(".glade-global-header");
      const tabs = document.querySelector("[data-glade-home-mode-tabs]");
      const builderLink = document.querySelector("[data-glade-lab-builder-link]");
      const preview = document.querySelector(".glade-lab-preview");
      return {
        homeHeaderCount: document.querySelectorAll(".glade-home-header").length,
        tabsInGlobalHeader: Boolean(globalHeader && tabs && globalHeader.contains(tabs)),
        builderInGlobalHeader: Boolean(globalHeader && builderLink && globalHeader.contains(builderLink)),
        previewTopOffset: home && preview ? preview.getBoundingClientRect().top - home.getBoundingClientRect().top : -1,
        contextVisible: Boolean(document.querySelector("[data-glade-context-panel]")?.getBoundingClientRect().width),
      };
    });
    assert.equal(homeChrome.homeHeaderCount, 0);
    assert.equal(homeChrome.tabsInGlobalHeader, true);
    assert.equal(homeChrome.builderInGlobalHeader, true);
    assert.ok(homeChrome.previewTopOffset <= 4, `home preview offset = ${homeChrome.previewTopOffset}`);
    assert.equal(homeChrome.contextVisible, false);
    assert.equal(await page.locator("[data-glade-home-mode-tabs]").count(), 1);
    assert.equal(await page.locator('[data-glade-home-mode-tab="lab"]').getAttribute("aria-selected"), "true");
    assert.equal(await page.locator('[data-glade-home-mode-tab="workbench"]').getAttribute("aria-selected"), "false");
    assert.equal(await page.locator('[data-glade-home-panel="lab"]').isVisible(), true);
    assert.equal(await page.locator('[data-glade-home-panel="workbench"]').isVisible(), false);
    assert.ok(await page.locator('[data-glade-lab-builder-link][href="/lwc/builder"]').count());

    await page.locator('[data-glade-home-mode-tab="workbench"]').click();
    assert.equal(await page.locator('[data-glade-home-mode-tab="lab"]').getAttribute("aria-selected"), "false");
    assert.equal(await page.locator('[data-glade-home-mode-tab="workbench"]').getAttribute("aria-selected"), "true");
    assert.equal(await page.locator('[data-glade-home-panel="lab"]').isVisible(), false);
    assert.equal(await page.locator('[data-glade-home-panel="workbench"]').isVisible(), true);
    assert.ok(await page.locator('[data-glade-page-workbench] [data-glade-home-route][href="/lwc/preview/tab/Lwc_Probe"]').count());
	    await page.locator('[data-glade-home-mode-tab="lab"]').click();
	    assert.equal(await page.locator('[data-glade-home-panel="lab"]').isVisible(), true);
	    assert.equal(await page.locator('[data-glade-home-panel="workbench"]').isVisible(), false);
	    await page.locator('[data-glade-lab-view-option="setup"]').click();
	    assert.equal(await page.locator('[data-glade-lab-view-option="setup"]').getAttribute("aria-pressed"), "true");
	    assert.equal(await page.locator('[data-glade-lab-view-option="preview"]').getAttribute("aria-pressed"), "false");
	    assert.equal(await page.locator("[data-glade-lab-components-rail]").isVisible(), true);
	    assert.equal(await page.locator("[data-glade-lab-props-rail]").isVisible(), true);

	    const model = JSON.parse(await page.locator("#glade-lwc-workbench").textContent());
	    const contextProbe = model.components.find((component) => component.qualifiedName === "c:contextProbe");
    assert.ok(contextProbe, "missing contextProbe in workbench model");
    assert.ok(contextProbe.apiProperties.some((prop) => prop.name === "title" && prop.default === "Local Shell Context"));
    assert.ok(contextProbe.apiProperties.some((prop) => prop.name === "recordId"));

    await page.locator('[data-glade-lab-component-option][data-glade-component="c:contextProbe"]').click();
    await page.locator('[data-glade-lab-host] c-context-probe').waitFor({ timeout: 60000 });
    assert.match(await page.locator('[data-glade-lab-host] c-context-probe').innerText({ timeout: 60000 }), /Local Shell Context/);
    assert.equal(await page.locator('[data-glade-lab-prop="title"]').inputValue(), "Local Shell Context");
    assert.equal(await page.locator('[data-glade-lab-route-link]').getAttribute("href"), "/lwc/preview/component/c/contextProbe");

    const contextEditorLayout = await page.evaluate(() => {
      const context = document.querySelector("[data-glade-lab-context]");
      const propList = document.querySelector("[data-glade-lab-prop-list]");
      return {
	        contextCount: document.querySelectorAll("[data-glade-lab-context]").length,
	        contextBeforeProps: Boolean(context && propList && context.getBoundingClientRect().bottom <= propList.getBoundingClientRect().top),
	        targetValue: document.querySelector("[data-glade-lab-context-kind]")?.value || "",
	        recordGroupHidden: document.querySelector('[data-glade-lab-context-group="record"]')?.hidden ?? false,
	        stateGroupVisible: !document.querySelector('[data-glade-lab-context-group="state"]')?.hidden,
	        stripText: document.querySelector("[data-glade-lab-context-strip]")?.textContent || "",
	      };
	    });
	    assert.equal(contextEditorLayout.contextCount, 1);
	    assert.equal(contextEditorLayout.contextBeforeProps, true);
	    assert.equal(contextEditorLayout.targetValue, "recordPage");
	    assert.equal(contextEditorLayout.recordGroupHidden, false);
	    assert.equal(contextEditorLayout.stateGroupVisible, true);
	    assert.match(contextEditorLayout.stripText, /c:contextProbe/);
	    assert.match(contextEditorLayout.stripText, /Record page/);
	    assert.match(contextEditorLayout.stripText, /Account/);

	    await page.locator("[data-glade-lab-context-kind]").selectOption("recordPage");
	    assert.equal(await page.locator('[data-glade-lab-context-group="record"]').isVisible(), true);
    const labObjectCombobox = page.locator("[data-glade-lab-context-object]");
    const labRecordCombobox = page.locator("[data-glade-lab-context-record]");
    assert.equal(await labObjectCombobox.getAttribute("role"), "combobox");
    assert.equal(await labRecordCombobox.getAttribute("role"), "combobox");
    await labObjectCombobox.fill("acc");
    await page.locator('[data-glade-lab-context-object-result][data-glade-api-name="Account"]').waitFor({ timeout: 60000 });
    await page.locator('[data-glade-lab-context-object-result][data-glade-api-name="Account"]').click();
    assert.equal(await labObjectCombobox.inputValue(), "Account");
    await labRecordCombobox.fill("preview");
    await page.locator('[data-glade-lab-context-record-result][data-glade-record-id="001000000000001AAA"]').waitFor({ timeout: 60000 });
    await page.locator('[data-glade-lab-context-record-result][data-glade-record-id="001000000000001AAA"]').click();
    await page.locator("[data-glade-lab-context-state-key]").fill("c__mode");
    await page.locator("[data-glade-lab-context-state-value]").fill("detail");
    await page.waitForFunction(() => {
      const context = JSON.parse(document.querySelector("script#glade-lwc-context").textContent);
      return context.kind === "recordPage" && context.recordId === "001000000000001AAA" && context.state?.c__mode === "detail";
    }, { timeout: 60000 });
    const recordPageContext = await page.evaluate(() => {
      const context = JSON.parse(document.querySelector("script#glade-lwc-context").textContent);
      const config = JSON.parse(document.querySelector("script#glade-lightning-config").textContent);
      return { context, pageReference: config.pageReference };
    });
    assert.equal(recordPageContext.context.kind, "recordPage");
    assert.equal(recordPageContext.context.objectApiName, "Account");
    assert.equal(recordPageContext.context.recordId, "001000000000001AAA");
    assert.equal(recordPageContext.context.state.c__mode, "detail");
	    assert.equal(recordPageContext.pageReference.type, "standard__recordPage");
	    assert.equal(recordPageContext.pageReference.attributes.objectApiName, "Account");
	    assert.equal(recordPageContext.pageReference.attributes.recordId, "001000000000001AAA");
	    assert.equal(recordPageContext.pageReference.state.c__mode, "detail");
	    assert.equal(await page.locator("[data-glade-lab-context-record]").inputValue(), "001000000000001AAA");
	    await page.locator('[data-glade-lab-strip-part="object"]').click();
	    await page.locator('[data-glade-lab-context-object-result][data-glade-api-name="Account"]').waitFor({ timeout: 60000 });
	    assert.equal(await page.locator("[data-glade-lab-context-object-results]").isHidden(), false);
	    await page.locator('[data-glade-lab-strip-part="record"]').click();
	    await page.locator('[data-glade-lab-context-record-result][data-glade-record-id="001000000000001AAA"]').waitFor({ timeout: 60000 });
	    assert.equal(await page.locator("[data-glade-lab-context-object-results]").isHidden(), true);
	    assert.equal(await page.locator("[data-glade-lab-context-record-results]").isHidden(), false);
	    await page.locator('[data-glade-lab-strip-part="formFactor"]').click();
	    assert.equal(await page.locator("[data-glade-lab-context-record-results]").isHidden(), true);
	    await page.locator('[data-glade-lab-strip-part="record"]').click();
	    assert.equal(await page.evaluate(() => document.activeElement?.matches("[data-glade-lab-context-record]")), true);

	    await page.locator("[data-glade-lab-context-kind]").selectOption("appPage");
	    await page.locator("[data-glade-lab-context-app]").fill("Developer_Workbench");
    await page.waitForFunction(() => {
      const context = JSON.parse(document.querySelector("script#glade-lwc-context").textContent);
      return context.kind === "appPage" && context.appName === "Developer_Workbench" && !context.recordId;
    }, { timeout: 60000 });
    const appPageContext = await page.evaluate(() => {
      const context = JSON.parse(document.querySelector("script#glade-lwc-context").textContent);
      const config = JSON.parse(document.querySelector("script#glade-lightning-config").textContent);
      return {
        context,
        pageReference: config.pageReference,
        recordGroupHidden: document.querySelector('[data-glade-lab-context-group="record"]')?.hidden ?? false,
      };
    });
    assert.equal(appPageContext.context.kind, "appPage");
    assert.equal(appPageContext.context.recordId, "");
    assert.equal(appPageContext.context.objectApiName, "");
    assert.equal(appPageContext.pageReference.type, "standard__app");
	    assert.equal(appPageContext.pageReference.attributes.appTarget, "Developer_Workbench");
	    assert.equal(appPageContext.recordGroupHidden, true);

	    await page.locator('[data-glade-lab-prop="title"]').fill("Focused Lab Title");
	    await page.waitForFunction(() => {
	      const host = document.querySelector("[data-glade-lab-host] c-context-probe");
      return host && host.innerText.includes("Focused Lab Title");
    }, { timeout: 60000 });
    assert.match(await page.locator('[data-glade-lab-host] c-context-probe').innerText(), /Focused Lab Title/);
    await page.locator("[data-glade-lab-reset-props]").click();
    await page.waitForFunction(() => {
      const host = document.querySelector("[data-glade-lab-host] c-context-probe");
      return host && host.innerText.includes("Local Shell Context");
    }, { timeout: 60000 });
	    assert.equal(await page.locator('[data-glade-lab-prop="title"]').inputValue(), "Local Shell Context");

	    await page.locator('[data-glade-lab-component-option][data-glade-component="c:actionProbe"]').click();
	    await page.waitForFunction(() => {
	      const context = JSON.parse(document.querySelector("script#glade-lwc-context").textContent);
	      return context.componentName === "c:actionProbe" && context.kind === "recordPage";
	    }, { timeout: 60000 });
	    const actionPropNames = await page.evaluate(() =>
	      Array.from(document.querySelectorAll("[data-glade-lab-prop]")).map((input) => input.dataset.gladeLabProp),
	    );
	    assert.ok(actionPropNames.includes("title"), actionPropNames.join(","));
	    assert.ok(actionPropNames.includes("actionName"), actionPropNames.join(","));
	    assert.ok(actionPropNames.includes("actionType"), actionPropNames.join(","));
	    for (const contextProp of ["recordId", "objectApiName", "formFactor", "state", "pageReference"]) {
	      assert.equal(actionPropNames.includes(contextProp), false, actionPropNames.join(","));
	    }
	    assert.equal(await page.locator("[data-glade-lab-context-kind]").inputValue(), "recordPage");
	    assert.equal(await page.locator("[data-glade-lab-context-object]").inputValue(), "Account");
	    assert.equal(await page.locator("[data-glade-lab-context-record]").inputValue(), "001000000000001AAA");
	    const recordSearchCountBeforeObjectClear = recordSearchURLs.length;
	    await page.locator("[data-glade-lab-context-object]").fill("");
	    await page.waitForFunction(() => {
	      const results = document.querySelector("[data-glade-lab-context-object-results]");
	      return results && !results.hidden && results.childElementCount > 0;
	    }, null, { timeout: 60000 });
	    assert.equal(await page.locator("[data-glade-lab-context-record]").inputValue(), "");
	    assert.equal(await page.locator("[data-glade-lab-context-record-results]").isVisible(), false);
	    assert.equal(await page.locator("[data-glade-lab-context-record]").getAttribute("aria-expanded"), "false");
	    assert.equal(recordSearchURLs.length, recordSearchCountBeforeObjectClear);
	    await page.locator("[data-glade-lab-context-object]").fill("acc");
	    await page.locator('[data-glade-lab-context-object-result][data-glade-api-name="Account"]').click();
	    await page.locator("[data-glade-lab-context-record]").fill("preview");
	    await page.locator('[data-glade-lab-context-record-result][data-glade-record-id="001000000000001AAA"]').click();
	    const searchedContext = await page.evaluate(() => JSON.parse(document.querySelector("script#glade-lwc-context").textContent));
	    assert.equal(searchedContext.objectApiName, "Account");
	    assert.equal(searchedContext.recordId, "001000000000001AAA");

	    await page.locator('[data-glade-lab-component-option][data-glade-component="c:contextProbe"]').click();
	    await page.waitForFunction(() => {
	      const context = JSON.parse(document.querySelector("script#glade-lwc-context").textContent);
	      return context.componentName === "c:contextProbe" && context.kind === "appPage";
	    }, { timeout: 60000 });
	    assert.equal(await page.locator("[data-glade-lab-context-app]").inputValue(), "Developer_Workbench");

	    await page.locator("[data-glade-lab-component-search]").fill("layout");
	    assert.equal(await page.locator('[data-glade-lab-component-option][data-glade-component="c:layoutProbe"]').isVisible(), true);
	    await page.locator('[data-glade-lab-component-option][data-glade-component="c:layoutProbe"]').click();
	    await page.locator('[data-glade-lab-host] c-layout-probe').waitFor({ timeout: 60000 });
	    assert.equal(await page.locator('[data-glade-lab-prop="objectApiName"]').count(), 0);
	    assert.equal(await page.locator('[data-glade-lab-prop="formFactor"]').count(), 0);
	    assert.equal(await page.locator("[data-glade-lab-selected]").innerText(), "c:layoutProbe");
	    assert.equal(await page.locator('[data-glade-lab-form-factor-option="Large"]').getAttribute("aria-pressed"), "true");
	    await page.locator('[data-glade-lab-form-factor-option="Small"]').click();
	    await page.waitForFunction(() => {
	      const context = JSON.parse(document.querySelector("script#glade-lwc-context").textContent);
	      return context.formFactor === "Small";
	    }, null, { timeout: 60000 });
	    const smallViewport = await page.evaluate(() => {
	      const context = JSON.parse(document.querySelector("script#glade-lwc-context").textContent);
	      const host = document.querySelector("[data-glade-lab-host]");
      const shell = document.querySelector("[data-glade-lab-host-shell]");
      return {
        contextFormFactor: context.formFactor,
        hostWidth: host.getBoundingClientRect().width,
        shellWidth: shell.getBoundingClientRect().width,
        pressedSmall: document.querySelector('[data-glade-lab-form-factor-option="Small"]')?.getAttribute("aria-pressed"),
        pressedLarge: document.querySelector('[data-glade-lab-form-factor-option="Large"]')?.getAttribute("aria-pressed"),
      };
    });
    assert.equal(smallViewport.contextFormFactor, "Small");
    assert.equal(smallViewport.pressedSmall, "true");
    assert.equal(smallViewport.pressedLarge, "false");
    assert.ok(smallViewport.hostWidth <= 420, `small host width = ${smallViewport.hostWidth}`);
    assert.ok(smallViewport.shellWidth > smallViewport.hostWidth, `shell ${smallViewport.shellWidth}, host ${smallViewport.hostWidth}`);
	    const labContext = await page.evaluate(() => JSON.parse(document.querySelector("script#glade-lwc-context").textContent));
	    assert.equal(labContext.componentName, "c:layoutProbe");
	    assert.equal(labContext.formFactor, "Small");
	    assert.equal(await page.locator('[data-glade-lab-fit-option="fit"]').count(), 1);
	    await page.locator('[data-glade-lab-fit-option="full"]').click();
	    const fitMetrics = await page.evaluate(() => {
	      const shell = document.querySelector("[data-glade-lab-host-shell]");
	      return {
	        fitMode: shell?.dataset.gladeFitMode || "",
	        fullPressed: document.querySelector('[data-glade-lab-fit-option="full"]')?.getAttribute("aria-pressed") || "",
	        fitPressed: document.querySelector('[data-glade-lab-fit-option="fit"]')?.getAttribute("aria-pressed") || "",
	      };
	    });
	    assert.equal(fitMetrics.fitMode, "full");
	    assert.equal(fitMetrics.fullPressed, "true");
	    assert.equal(fitMetrics.fitPressed, "false");

    const setupView = await page.evaluate(() => {
      const visible = (selector) => {
        const node = document.querySelector(selector);
        return Boolean(node && node.getBoundingClientRect().width > 0 && node.getBoundingClientRect().height > 0);
      };
      return {
        labView: document.querySelector("[data-glade-component-lab]")?.dataset.gladeLabView || "",
        previewWidth: document.querySelector(".glade-lab-preview")?.getBoundingClientRect().width || 0,
        componentRailVisible: visible("[data-glade-lab-components-rail]"),
        propsRailVisible: visible("[data-glade-lab-props-rail]"),
        contextVisible: visible("[data-glade-context-panel]"),
        setupPressed: document.querySelector('[data-glade-lab-view-option="setup"]')?.getAttribute("aria-pressed") || "",
        previewPressed: document.querySelector('[data-glade-lab-view-option="preview"]')?.getAttribute("aria-pressed") || "",
        documentVerticalOverflow: document.documentElement.scrollHeight > document.documentElement.clientHeight + 1,
      };
    });
    assert.equal(setupView.labView, "setup");
    assert.equal(setupView.componentRailVisible, true);
    assert.equal(setupView.propsRailVisible, true);
    assert.equal(setupView.contextVisible, false);
    assert.equal(setupView.setupPressed, "true");
    assert.equal(setupView.previewPressed, "false");
    assert.equal(setupView.documentVerticalOverflow, false);

    await page.locator('[data-glade-lab-view-option="preview"]').click();
    const previewView = await page.evaluate(() => {
      const visible = (selector) => {
        const node = document.querySelector(selector);
        return Boolean(node && node.getBoundingClientRect().width > 0 && node.getBoundingClientRect().height > 0);
      };
      return {
        labView: document.querySelector("[data-glade-component-lab]")?.dataset.gladeLabView || "",
        workbenchView: document.querySelector("[data-glade-workbench-console]")?.dataset.gladeLabView || "",
        previewWidth: document.querySelector(".glade-lab-preview")?.getBoundingClientRect().width || 0,
        componentRailVisible: visible("[data-glade-lab-components-rail]"),
        propsRailVisible: visible("[data-glade-lab-props-rail]"),
        contextVisible: visible("[data-glade-context-panel]"),
        setupPressed: document.querySelector('[data-glade-lab-view-option="setup"]')?.getAttribute("aria-pressed") || "",
        previewPressed: document.querySelector('[data-glade-lab-view-option="preview"]')?.getAttribute("aria-pressed") || "",
        documentVerticalOverflow: document.documentElement.scrollHeight > document.documentElement.clientHeight + 1,
      };
    });
    assert.equal(previewView.labView, "preview");
    assert.equal(previewView.workbenchView, "preview");
    assert.equal(previewView.componentRailVisible, false);
    assert.equal(previewView.propsRailVisible, false);
    assert.equal(previewView.contextVisible, false);
    assert.equal(previewView.setupPressed, "false");
    assert.equal(previewView.previewPressed, "true");
    assert.equal(previewView.documentVerticalOverflow, false);
    assert.ok(previewView.previewWidth > setupView.previewWidth + 300, `preview ${previewView.previewWidth}, setup ${setupView.previewWidth}`);

    await page.locator('[data-glade-lab-view-option="setup"]').click();
    assert.equal(await page.locator('[data-glade-lab-view-option="setup"]').getAttribute("aria-pressed"), "true");
    assert.equal(await page.locator("[data-glade-context-panel]").isVisible(), false);

    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(`${server.baseURL}/lwc`, { waitUntil: "networkidle" });
    await page.locator("[data-glade-component-lab]").waitFor({ timeout: 60000 });
    const mobileSetup = await page.evaluate(() => {
      const box = (selector) => document.querySelector(selector)?.getBoundingClientRect();
      const preview = box(".glade-lab-preview");
      const components = box("[data-glade-lab-components-rail]");
      const props = box("[data-glade-lab-props-rail]");
      return {
        previewWidth: preview?.width || 0,
        previewTop: preview?.top || 0,
        componentsTop: components?.top || 0,
        propsTop: props?.top || 0,
        documentHorizontalOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
      };
    });
    assert.ok(mobileSetup.previewWidth >= 320, `mobile preview width = ${mobileSetup.previewWidth}`);
    assert.ok(mobileSetup.componentsTop > mobileSetup.previewTop, `components top ${mobileSetup.componentsTop}, preview top ${mobileSetup.previewTop}`);
    assert.ok(mobileSetup.propsTop > mobileSetup.componentsTop, `props top ${mobileSetup.propsTop}, components top ${mobileSetup.componentsTop}`);
    assert.equal(mobileSetup.documentHorizontalOverflow, false);
  } finally {
    await browser.close();
    await server.close();
  }

  assert.deepEqual(consoleErrors, []);
  assert.deepEqual(pageErrors, []);
});

test("LWC shell workbench lets developers compose a local page from available components", async (t) => {
  const server = await startLWCDevServer(t, {
    projectRel: "testdata/local-tests/lwc-shell",
    pagePath: "/lwc/builder",
  });
  if (!server) {
    return;
  }

  const browser = await chromium.launch({ headless: true });
  const consoleErrors = [];
  const pageErrors = [];
  try {
    const page = await browser.newPage();
    page.on("console", (msg) => {
      if (msg.type() === "error") {
        consoleErrors.push(msg.text());
      }
    });
    page.on("pageerror", (err) => {
      pageErrors.push(err.message);
    });

    await page.goto(`${server.baseURL}/lwc/builder`, { waitUntil: "networkidle" });

    await page.locator("[data-glade-workbench-builder]").waitFor({ timeout: 60000 });
    assert.ok(await page.locator('[data-glade-home-link][href="/lwc"]').count());
    await page.locator("[data-glade-component-search]").fill("context");
    assert.equal(await page.locator('[data-glade-component-card][data-glade-component="c:contextProbe"]').isVisible(), true);
    assert.equal(await page.locator('[data-glade-component-card][data-glade-component="c:recordProbe"]').isVisible(), false);
    await page.locator("[data-glade-page-kind]").selectOption("recordPage");
    await page.locator("[data-glade-object-input]").fill("Account");
    await page.locator("[data-glade-record-input]").fill("001000000000001AAA");
    await page.locator("[data-glade-component-search]").fill("recordProbe");
    await page.locator('[data-glade-add-component="c:recordProbe"][data-glade-region="main"]').click();
    await page.locator('[data-glade-draft-component="c:recordProbe"]').waitFor({ timeout: 60000 });
    await page.locator("[data-glade-page-kind]").selectOption("appPage");
    assert.equal(await page.locator('[data-glade-draft-component="c:recordProbe"]').count(), 0);
    assert.match(await page.locator("[data-glade-draft-status]").innerText(), /^0 placed/);
    await page.locator("[data-glade-page-kind]").selectOption("recordPage");
    await page.locator("[data-glade-component-search]").fill("context");
    await page.locator('[data-glade-add-component="c:contextProbe"][data-glade-region="main"]').click();

    await page.locator('[data-glade-draft-component="c:contextProbe"]').waitFor({ timeout: 60000 });
    await page.locator("c-context-probe").waitFor({ timeout: 60000 });
    const text = await page.locator("c-context-probe").innerText({ timeout: 60000 });
    assert.match(text, /Local Shell Context/);
    assert.match(text, /001000000000001AAA/);
    assert.doesNotMatch(text, /001000000000999AAA/);
    const contextPanel = await page.locator("[data-glade-context-panel]").innerText({ timeout: 60000 });
    assert.match(contextPanel, /standard__recordPage/);
    assert.match(contextPanel, /Account/);
    assert.match(contextPanel, /001000000000001AAA/);
    assert.equal(await page.locator(".glade-route-menu[open]").count(), 0);
    assert.equal(await page.locator(".glade-stage > .glade-region").count(), 0);

    const model = JSON.parse(await page.locator("#glade-lwc-workbench").textContent());
    assert.ok(model.components.some((component) => component.qualifiedName === "c:contextProbe"));
    assert.equal(await page.locator('[data-glade-page-canvas] [data-glade-region-drop="main"]').count(), 1);

    await page.locator("[data-glade-layout-picker]").selectOption("single");
    assert.equal(await page.locator("[data-glade-page-layout]").getAttribute("data-glade-layout"), "single");
    assert.equal(await page.locator('[data-glade-region-drop="sidebar"]').isVisible(), false);

    await page.locator("[data-glade-layout-picker]").selectOption("mainSidebar");
    await page.locator("[data-glade-component-search]").fill("context");
    await page.evaluate(() => {
      const card = document.querySelector('[data-glade-component-card][data-glade-component="c:contextProbe"]');
      const drop = document.querySelector('[data-glade-region-drop="sidebar"]');
      const data = new DataTransfer();
      card.dispatchEvent(new DragEvent("dragstart", { bubbles: true, dataTransfer: data }));
      drop.dispatchEvent(new DragEvent("dragover", { bubbles: true, dataTransfer: data }));
      drop.dispatchEvent(new DragEvent("drop", { bubbles: true, dataTransfer: data }));
    });
    await page.locator('[data-glade-region-items="sidebar"] [data-glade-draft-component="c:contextProbe"]').waitFor({ timeout: 60000 });
  } finally {
    await browser.close();
    await server.close();
  }

  assert.deepEqual(consoleErrors, []);
  assert.deepEqual(pageErrors, []);
});

test("LWC shell workbench keeps the root builder dense on desktop", async (t) => {
  const server = await startLWCDevServer(t, {
    projectRel: "testdata/local-tests/lwc-shell",
    pagePath: "/lwc/builder",
  });
  if (!server) {
    return;
  }

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1440, height: 900 } });
    await page.goto(`${server.baseURL}/lwc/builder`, { waitUntil: "networkidle" });
    await page.locator("[data-glade-workbench-builder]").waitFor({ timeout: 60000 });

    const metrics = await page.evaluate(() => {
      const box = (selector) => document.querySelector(selector).getBoundingClientRect();
      return {
        builderActive: document.body.getAttribute("data-glade-builder-active"),
        commandbarHeight: box(".glade-builder-commandbar").height,
        commandbarOverflow: document.querySelector(".glade-builder-commandbar").scrollWidth >
          document.querySelector(".glade-builder-commandbar").clientWidth + 1,
        outsideContextDisplay: getComputedStyle(document.querySelector(".glade-context-panel")).display,
        layoutColumns: getComputedStyle(document.querySelector(".glade-builder-layout")).gridTemplateColumns.split(" ").length,
        paletteWidth: box(".glade-builder-palette").width,
        canvasShellWidth: box(".glade-builder-canvas-shell").width,
        propertiesWidth: box(".glade-builder-properties").width,
        selectedLayout: document.querySelector("[data-glade-layout-picker]")?.value,
        canvasColumns: getComputedStyle(document.querySelector(".glade-page-canvas")).gridTemplateColumns.split(" ").length,
        consoleControlDisplay: getComputedStyle(document.querySelector(".glade-checkbox-control")).display,
        consoleInputWidth: box(".glade-checkbox-control input").width,
        mediumButtonWidth: box('[data-glade-form-factor-option="Medium"]').width,
        routePickerCount: document.querySelectorAll("details.glade-route-picker").length,
        routeMenuOpen: document.querySelector("details.glade-route-menu")?.open ?? true,
        routeMenuTop: box(".glade-route-menu").top,
        routeMenuWidth: box(".glade-route-menu").width,
        builderTop: box("[data-glade-workbench-builder]").top,
        pageTitleText: document.querySelector("[data-glade-draft-title]")?.textContent?.trim(),
        pageTitleClientWidth: document.querySelector("[data-glade-draft-title]")?.clientWidth ?? 0,
        pageTitleScrollWidth: document.querySelector("[data-glade-draft-title]")?.scrollWidth ?? 0,
        pageTitleElement: document.querySelector("[data-glade-draft-title]")?.tagName,
        pageTitleLeft: box("[data-glade-draft-title]").left,
        routeMenuRight: box(".glade-route-menu").right,
        maxCatalogActionHeight: Math.max(
          ...Array.from(document.querySelectorAll(".glade-component-actions .glade-shell-button")).map((button) =>
            button.getBoundingClientRect().height,
          ),
        ),
        catalogActionLabels: Array.from(document.querySelectorAll(".glade-component-actions .glade-shell-button")).map((button) =>
          button.textContent.trim(),
        ),
        inertButtonCount: Array.from(document.querySelectorAll("button:disabled")).filter((button) =>
          /Analyze|Activation|Save|Cut|Copy|Paste|Fields|Help/.test(button.textContent),
        ).length,
        horizontalOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      };
    });

    assert.equal(metrics.horizontalOverflow, false);
    assert.equal(metrics.commandbarOverflow, false);
    assert.equal(metrics.builderActive, "true");
    assert.equal(metrics.outsideContextDisplay, "none");
    assert.equal(metrics.routePickerCount, 0);
    assert.equal(metrics.routeMenuOpen, false);
    assert.ok(metrics.routeMenuTop <= metrics.builderTop + 12, `route menu top = ${metrics.routeMenuTop}, builder top = ${metrics.builderTop}`);
    assert.ok(metrics.routeMenuWidth <= 160, `route menu width = ${metrics.routeMenuWidth}`);
    assert.equal(metrics.pageTitleText, "Draft App Page");
    assert.equal(metrics.pageTitleElement, "SPAN");
    assert.ok(metrics.pageTitleLeft - metrics.routeMenuRight >= 24, `title inset = ${metrics.pageTitleLeft - metrics.routeMenuRight}`);
    assert.ok(metrics.pageTitleScrollWidth <= metrics.pageTitleClientWidth + 1, `title scroll ${metrics.pageTitleScrollWidth}, client ${metrics.pageTitleClientWidth}`);
    assert.equal(metrics.layoutColumns, 3);
    assert.equal(metrics.inertButtonCount, 0);
    assert.ok(metrics.maxCatalogActionHeight <= 28, `catalog action height = ${metrics.maxCatalogActionHeight}`);
    assert.ok(metrics.commandbarHeight <= 52, `commandbar height = ${metrics.commandbarHeight}`);
    assert.ok(metrics.paletteWidth <= 295, `palette width = ${metrics.paletteWidth}`);
    assert.ok(metrics.propertiesWidth <= 360, `properties width = ${metrics.propertiesWidth}`);
    assert.ok(metrics.canvasShellWidth > metrics.paletteWidth, `canvas ${metrics.canvasShellWidth} palette ${metrics.paletteWidth}`);
    assert.ok(metrics.canvasShellWidth > metrics.propertiesWidth, `canvas ${metrics.canvasShellWidth} properties ${metrics.propertiesWidth}`);
    assert.equal(metrics.selectedLayout, "single");
    assert.equal(metrics.canvasColumns, 1);
    assert.equal(metrics.consoleControlDisplay, "flex");
    assert.ok(metrics.consoleInputWidth <= 20, `console input width = ${metrics.consoleInputWidth}`);
    assert.ok(metrics.mediumButtonWidth >= 64, `medium button width = ${metrics.mediumButtonWidth}`);
    assert.equal(metrics.catalogActionLabels.every((label) => !label.startsWith("+")), true);
    const noSidecar = await page.evaluate(() => ({
      mobileSidecarCount: document.querySelectorAll("[data-glade-mobile-sidecar], .glade-mobile-frame").length,
      previewCanvasCount: document.querySelectorAll("[data-glade-preview-canvas]").length,
    }));
    assert.equal(noSidecar.mobileSidecarCount, 0);
    assert.equal(noSidecar.previewCanvasCount, 1);

    assert.equal(await page.locator("[data-glade-builder-view-tabs]").count(), 1);
    assert.equal(await page.locator('[data-glade-builder-view-option="setup"]').getAttribute("aria-pressed"), "true");
    assert.equal(await page.locator('[data-glade-builder-view-option="preview"]').getAttribute("aria-pressed"), "false");
    await page.locator('[data-glade-add-component="c:contextProbe"][data-glade-region="main"]').click();
    await page.locator('[data-glade-draft-component="c:contextProbe"]').waitFor({ timeout: 60000 });
    const setupPreviewMetrics = await page.evaluate(() => {
      const box = (selector) => document.querySelector(selector)?.getBoundingClientRect();
      return {
        builderView: document.querySelector("[data-glade-workbench-builder]")?.dataset.gladeBuilderView || "",
        layoutDisplay: getComputedStyle(document.querySelector(".glade-builder-layout")).display,
        paletteVisible: Boolean(box(".glade-builder-palette")?.width),
        propertiesVisible: Boolean(box(".glade-builder-properties")?.width),
        canvasWidth: box(".glade-builder-canvas-shell")?.width || 0,
        regionHeadingVisible: Boolean(box(".glade-draft-region h3")?.height),
        draftHeaderVisible: Boolean(box(".glade-draft-component > header")?.height),
      };
    });
    assert.equal(setupPreviewMetrics.builderView, "setup");
    assert.equal(setupPreviewMetrics.paletteVisible, true);
    assert.equal(setupPreviewMetrics.propertiesVisible, true);
    assert.equal(setupPreviewMetrics.regionHeadingVisible, true);
    assert.equal(setupPreviewMetrics.draftHeaderVisible, true);

    await page.locator('[data-glade-builder-view-option="preview"]').click();
    const builderPreviewMetrics = await page.evaluate(() => {
      const box = (selector) => document.querySelector(selector)?.getBoundingClientRect();
      return {
        builderView: document.querySelector("[data-glade-workbench-builder]")?.dataset.gladeBuilderView || "",
        layoutDisplay: getComputedStyle(document.querySelector(".glade-builder-layout")).display,
        paletteVisible: Boolean(box(".glade-builder-palette")?.width),
        propertiesVisible: Boolean(box(".glade-builder-properties")?.width),
        canvasWidth: box(".glade-builder-canvas-shell")?.width || 0,
        regionHeadingVisible: Boolean(box(".glade-draft-region h3")?.height),
        draftHeaderVisible: Boolean(box(".glade-draft-component > header")?.height),
        componentVisible: Boolean(box('[data-glade-draft-component="c:contextProbe"]')?.height),
        horizontalOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth + 1,
        verticalOverflow: document.documentElement.scrollHeight > document.documentElement.clientHeight + 1,
      };
    });
    assert.equal(builderPreviewMetrics.builderView, "preview");
    assert.equal(await page.locator('[data-glade-builder-view-option="setup"]').getAttribute("aria-pressed"), "false");
    assert.equal(await page.locator('[data-glade-builder-view-option="preview"]').getAttribute("aria-pressed"), "true");
    assert.equal(builderPreviewMetrics.layoutDisplay, "flex");
    assert.equal(builderPreviewMetrics.paletteVisible, false);
    assert.equal(builderPreviewMetrics.propertiesVisible, false);
    assert.equal(builderPreviewMetrics.regionHeadingVisible, false);
    assert.equal(builderPreviewMetrics.draftHeaderVisible, false);
    assert.equal(builderPreviewMetrics.componentVisible, true);
    assert.ok(builderPreviewMetrics.canvasWidth > setupPreviewMetrics.canvasWidth + 300, `preview ${builderPreviewMetrics.canvasWidth}, setup ${setupPreviewMetrics.canvasWidth}`);
    assert.equal(builderPreviewMetrics.horizontalOverflow, false);
    assert.equal(builderPreviewMetrics.verticalOverflow, false);

    await page.reload({ waitUntil: "networkidle" });
    await page.locator("[data-glade-workbench-builder]").waitFor({ timeout: 60000 });
    assert.equal(await page.locator("[data-glade-workbench-builder]").getAttribute("data-glade-builder-view"), "preview");
    assert.equal(await page.locator('[data-glade-builder-view-option="preview"]').getAttribute("aria-pressed"), "true");
    await page.locator('[data-glade-builder-view-option="setup"]').click();
    assert.equal(await page.locator("[data-glade-workbench-builder]").getAttribute("data-glade-builder-view"), "setup");
  } finally {
    await browser.close();
    await server.close();
  }
});

test("LWC shell workbench keeps the root builder compact on mobile", async (t) => {
  const server = await startLWCDevServer(t, {
    projectRel: "testdata/local-tests/lwc-shell",
    pagePath: "/lwc/builder",
  });
  if (!server) {
    return;
  }

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 390, height: 844 } });
    await page.goto(`${server.baseURL}/lwc/builder`, { waitUntil: "networkidle" });
    await page.locator("[data-glade-workbench-builder]").waitFor({ timeout: 60000 });

    const metrics = await page.evaluate(() => {
      const box = (selector) => document.querySelector(selector).getBoundingClientRect();
      return {
        commandbarHeight: box(".glade-builder-commandbar").height,
        commandbarOverflow: document.querySelector(".glade-builder-commandbar").scrollWidth >
          document.querySelector(".glade-builder-commandbar").clientWidth + 1,
        layoutColumns: getComputedStyle(document.querySelector(".glade-builder-layout")).gridTemplateColumns.split(" ").length,
        canvasColumns: getComputedStyle(document.querySelector(".glade-page-canvas")).gridTemplateColumns.split(" ").length,
        routePickerCount: document.querySelectorAll("details.glade-route-picker").length,
        routeMenuOpen: document.querySelector("details.glade-route-menu")?.open ?? true,
        horizontalOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      };
    });

    assert.equal(metrics.horizontalOverflow, false);
    assert.equal(metrics.commandbarOverflow, false);
    assert.equal(metrics.routePickerCount, 0);
    assert.equal(metrics.routeMenuOpen, false);
    assert.equal(metrics.layoutColumns, 1);
    assert.equal(metrics.canvasColumns, 1);
    assert.ok(metrics.commandbarHeight <= 120, `commandbar height = ${metrics.commandbarHeight}`);
  } finally {
    await browser.close();
    await server.close();
  }
});

test("LWC shell workbench builder preserves draft context across reload", async (t) => {
  const server = await startLWCDevServer(t, {
    projectRel: "testdata/local-tests/lwc-shell",
    pagePath: "/lwc/builder",
  });
  if (!server) {
    return;
  }

  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 820 } });
    await page.goto(`${server.baseURL}/lwc/builder`, { waitUntil: "networkidle" });
    await page.locator("[data-glade-workbench-builder]").waitFor({ timeout: 60000 });

    await page.locator("[data-glade-target-picker]").selectOption("recordPage");
    await page.locator('[data-glade-context-group="record"]').waitFor({ state: "visible", timeout: 60000 });
    await page.locator("[data-glade-object-selector]").fill("Account");
    await page.locator("[data-glade-record-input]").fill("001000000000001AAA");
    await page.locator("[data-glade-form-factor-option=\"Small\"]").click();
    await page.locator("[data-glade-component-search]").fill("context");
    await page.locator('[data-glade-add-component="c:contextProbe"][data-glade-region="main"]').click();
    await page.locator('[data-glade-region-items="main"] [data-glade-draft-component="c:contextProbe"]').waitFor({ timeout: 60000 });

    await page.reload({ waitUntil: "networkidle" });
    await page.locator("[data-glade-workbench-builder]").waitFor({ timeout: 60000 });
    await page.locator('[data-glade-context-group="record"]').waitFor({ state: "visible", timeout: 60000 });

    assert.equal(await page.locator("[data-glade-target-picker]").inputValue(), "recordPage");
    assert.equal(await page.locator("[data-glade-object-selector]").inputValue(), "Account");
    assert.equal(await page.locator("[data-glade-record-input]").inputValue(), "001000000000001AAA");
    assert.equal(await page.locator("select[data-glade-form-factor]").inputValue(), "Small");
    assert.equal(await page.locator("[data-glade-component-search]").inputValue(), "context");
    assert.equal(await page.locator('[data-glade-region-items="main"] [data-glade-draft-component="c:contextProbe"]').count(), 1);
    const context = await page.locator("script#glade-lwc-context").evaluate((node) => JSON.parse(node.textContent));
    assert.equal(context.kind, "recordPage");
    assert.equal(context.recordId, "001000000000001AAA");
    assert.equal(context.objectApiName, "Account");
  } finally {
    await browser.close();
    await server.close();
  }
	});

test("LWC shell workbench builder does not open object search on restored record context", async (t) => {
  const server = await startLWCDevServer(t, {
    projectRel: "testdata/local-tests/lwc-shell",
    pagePath: "/lwc/builder",
  });
  if (!server) {
    return;
  }

  const browser = await chromium.launch({ headless: true });
  let objectSearchRequests = 0;
  try {
    const page = await browser.newPage({ viewport: { width: 1280, height: 820 } });
    await page.route("**/lightning/local/objects.json?*", async (route) => {
      objectSearchRequests += 1;
      await route.continue();
    });
    await page.addInitScript(() => {
      sessionStorage.setItem("glade:workbench-builder:v1", JSON.stringify({
        kind: "recordPage",
        layout: "single",
        viewMode: "setup",
        objectApiName: "Account",
        recordId: "001000000000001AAA",
        appName: "Local",
        formFactor: "Large",
        components: [],
      }));
    });

    await page.goto(`${server.baseURL}/lwc/builder`, { waitUntil: "networkidle" });
    await page.locator("[data-glade-workbench-builder]").waitFor({ timeout: 60000 });
    await page.locator('[data-glade-context-group="record"]').waitFor({ state: "visible", timeout: 60000 });

    const bootSearchState = await page.evaluate(() => ({
      kind: document.querySelector("[data-glade-target-picker]")?.value || "",
      objectExpanded: document.querySelector("[data-glade-object-selector]")?.getAttribute("aria-expanded") || "",
      objectHidden: document.querySelector("[data-glade-object-results]")?.hidden ?? false,
      objectChildCount: document.querySelector("[data-glade-object-results]")?.childElementCount ?? -1,
      activeElementIsObject: document.activeElement?.matches("[data-glade-object-selector]") ?? false,
    }));
    assert.equal(bootSearchState.kind, "recordPage");
    assert.equal(bootSearchState.objectExpanded, "false");
    assert.equal(bootSearchState.objectHidden, true);
    assert.equal(bootSearchState.objectChildCount, 0);
    assert.equal(bootSearchState.activeElementIsObject, false);
    assert.equal(objectSearchRequests, 0);
  } finally {
    await browser.close();
    await server.close();
  }
});

test("LWC shell workbench builder exposes context controls", async (t) => {
  const server = await startLWCDevServer(t, {
    projectRel: "testdata/local-tests/lwc-shell",
    pagePath: "/lwc/builder",
  });
  if (!server) {
    return;
  }

  const browser = await chromium.launch({ headless: true });
  const consoleErrors = [];
  const pageErrors = [];
  try {
    const page = await browser.newPage();
    page.on("console", (msg) => {
      if (msg.type() === "error") {
        consoleErrors.push(msg.text());
      }
    });
    page.on("pageerror", (err) => {
      pageErrors.push(err.message);
    });

    await page.goto(`${server.baseURL}/lwc/builder`, { waitUntil: "networkidle" });

    await page.locator("[data-glade-workbench-builder]").waitFor({ timeout: 60000 });
    assert.equal(await page.locator('[data-glade-context-group="record"]').count(), 1);
    assert.equal(await page.locator('[data-glade-context-group="record"]').isHidden(), true);
    assert.equal(await page.locator('[data-glade-context-group="app"]').isVisible(), true);
    const appContext = await page.locator("script#glade-lwc-context").evaluate((node) => JSON.parse(node.textContent));
    assert.equal(appContext.kind, "appPage");
    assert.equal(appContext.recordId, "");
    assert.equal(appContext.objectApiName, "");
    const targetOptions = await page.locator("[data-glade-target-picker] option").evaluateAll((options) =>
      options.map((option) => option.value),
    );
    assert.deepEqual(
      ["utilityBar", "flowScreen", "flowAction"].every((value) => targetOptions.includes(value)),
      true,
    );
    await page.locator("[data-glade-target-picker]").selectOption("recordPage");
    await page.locator('[data-glade-context-group="record"]').waitFor({ state: "visible", timeout: 60000 });
    const objectCombobox = page.locator("[data-glade-object-selector]");
    const objectDropdown = page.locator("[data-glade-object-results]");
    const recordCombobox = page.locator("[data-glade-record-input]");
    const recordDropdown = page.locator("[data-glade-record-results]");
    assert.equal(await objectCombobox.getAttribute("role"), "combobox");
    assert.equal(await objectCombobox.getAttribute("aria-autocomplete"), "list");
    assert.equal(await objectCombobox.getAttribute("aria-expanded"), "false");
    assert.equal(await objectDropdown.getAttribute("role"), "listbox");
    assert.equal(await objectDropdown.isHidden(), true);
    await page.locator("[data-glade-object-selector]").fill("acc");
    await page.locator('[data-glade-object-result][data-glade-api-name="Account"]').waitFor({ timeout: 60000 });
    assert.equal(await objectCombobox.getAttribute("aria-expanded"), "true");
    const objectDropdownMetrics = await objectDropdown.evaluate((node) => {
      const style = getComputedStyle(node);
      return { maxHeight: Number.parseFloat(style.maxHeight), overflowY: style.overflowY };
    });
    assert.ok(objectDropdownMetrics.maxHeight <= 260, `object dropdown max-height = ${objectDropdownMetrics.maxHeight}`);
    assert.match(objectDropdownMetrics.overflowY, /auto|scroll/);
    await objectCombobox.press("ArrowDown");
    assert.equal(await page.evaluate(() => document.activeElement?.getAttribute("data-glade-api-name")), "Account");
    await page.keyboard.press("Escape");
    assert.equal(await objectCombobox.getAttribute("aria-expanded"), "false");
    await objectCombobox.fill("acc");
    await page.locator('[data-glade-object-result][data-glade-api-name="Account"]').waitFor({ timeout: 60000 });
    await page.locator('[data-glade-object-result][data-glade-api-name="Account"]').click();
    assert.equal(await objectCombobox.getAttribute("aria-expanded"), "false");
    assert.equal(await recordCombobox.getAttribute("role"), "combobox");
    assert.equal(await recordCombobox.getAttribute("aria-autocomplete"), "list");
    assert.equal(await recordCombobox.getAttribute("aria-expanded"), "false");
    assert.equal(await recordDropdown.getAttribute("role"), "listbox");
    assert.equal(await recordDropdown.isHidden(), true);
    await page.locator("[data-glade-record-input]").fill("local");
    await page.locator('[data-glade-record-result][data-glade-record-id="001000000000001AAA"]').waitFor({ timeout: 60000 });
    assert.equal(await recordCombobox.getAttribute("aria-expanded"), "true");
    const recordDropdownMetrics = await recordDropdown.evaluate((node) => {
      const style = getComputedStyle(node);
      return { maxHeight: Number.parseFloat(style.maxHeight), overflowY: style.overflowY };
    });
    assert.ok(recordDropdownMetrics.maxHeight <= 260, `record dropdown max-height = ${recordDropdownMetrics.maxHeight}`);
    assert.match(recordDropdownMetrics.overflowY, /auto|scroll/);
    await page.locator('[data-glade-record-result][data-glade-record-id="001000000000001AAA"]').click();
    assert.equal(await recordCombobox.getAttribute("aria-expanded"), "false");
    await objectCombobox.fill("");
    assert.equal(await recordCombobox.inputValue(), "");
    assert.equal(await recordDropdown.isHidden(), true);
    assert.equal(await recordCombobox.getAttribute("aria-expanded"), "false");
    await objectCombobox.fill("acc");
    await page.locator('[data-glade-object-result][data-glade-api-name="Account"]').click();
    await recordCombobox.fill("local");
    await page.locator('[data-glade-record-result][data-glade-record-id="001000000000001AAA"]').click();
    await page.locator("[data-glade-state-key]").fill("c__view");
    await page.locator("[data-glade-state-value]").fill("detail");
    await page.locator("[data-glade-community-selector]").fill("Partner_Portal");
    await page.locator("[data-glade-console-mode]").check();
    await page.locator("[data-glade-layout-picker]").selectOption("mainSidebar");
    await page.locator("[data-glade-component-search]").fill("context");
    await page.locator('[data-glade-add-component="c:contextProbe"][data-glade-region="sidebar"]').click();
    await page.locator('[data-glade-region-items="sidebar"] [data-glade-draft-component="c:contextProbe"]').waitFor({ timeout: 60000 });
    await page.locator('[data-glade-form-factor-option="Small"]').click();
    const viewportMetrics = await page.evaluate(() => {
      const canvas = document.querySelector("[data-glade-page-canvas]");
      const box = canvas.getBoundingClientRect();
      const sidebarAdd = document.querySelector(
        '[data-glade-component-card][data-glade-component="c:contextProbe"]:not([hidden]) [data-glade-add-component="c:contextProbe"][data-glade-region="sidebar"]',
      );
      sidebarAdd?.click();
      return {
        canvasFormFactorLabel: document.querySelector("[data-glade-canvas-form-factor]")?.textContent?.trim(),
        formFactor: canvas.dataset.gladeFormFactor,
        mainDraftCount: document.querySelectorAll('[data-glade-region-items="main"] [data-glade-draft-component="c:contextProbe"]').length,
        sidebarAddAriaDisabled: sidebarAdd?.getAttribute("aria-disabled"),
        sidebarAddDisabled: Boolean(sidebarAdd?.disabled),
        sidebarDraftCount: document.querySelectorAll('[data-glade-region-items="sidebar"] [data-glade-draft-component]').length,
        width: box.width,
        sidecarCount: document.querySelectorAll("[data-glade-mobile-sidecar], .glade-mobile-frame").length,
      };
    });
    assert.equal(viewportMetrics.formFactor, "Small");
    assert.equal(viewportMetrics.canvasFormFactorLabel, "Small");
    assert.equal(viewportMetrics.sidebarAddDisabled || viewportMetrics.sidebarAddAriaDisabled === "true", true);
    assert.equal(viewportMetrics.sidebarDraftCount, 0);
    assert.equal(viewportMetrics.mainDraftCount, 1);
    assert.equal(viewportMetrics.sidecarCount, 0);
    assert.ok(viewportMetrics.width <= 430, `small viewport width = ${viewportMetrics.width}`);

    const context = await page.locator("script#glade-lwc-context").evaluate((node) => JSON.parse(node.textContent));
    assert.equal(context.kind, "recordPage");
    assert.equal(context.recordId, "001000000000001AAA");
    assert.equal(context.formFactor, "Small");
    assert.equal(context.community.site, "Partner_Portal");
    assert.equal(context.state.c__view, "detail");
    assert.equal(await page.locator("[data-glade-component-picker]").count(), 1);
    assert.equal(await page.locator("[data-glade-flow-inputs]").count(), 1);
    assert.equal(await page.locator("[data-glade-app-selector]").count(), 1);
    assert.equal(await page.locator("[data-glade-object-selector]").count(), 1);
    await page.locator("[data-glade-target-picker]").selectOption("flowScreen");
    await page.locator("[data-glade-app-selector]").fill("Local_Flow");
    const flowContext = await page.locator("script#glade-lwc-context").evaluate((node) => JSON.parse(node.textContent));
    assert.equal(flowContext.kind, "flowScreen");
    assert.equal(flowContext.flow.apiName, "Local_Flow");
    assert.equal(flowContext.recordId, "");
    assert.equal(flowContext.objectApiName, "");
  } finally {
    await browser.close();
    await server.close();
  }

  assert.deepEqual(consoleErrors, []);
  assert.deepEqual(pageErrors, []);
});
