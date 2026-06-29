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

test("Workbench Console collects runtime events and switches debug tabs with aria-selected", async () => {
  const server = await startWorkbenchServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`, { waitUntil: "networkidle" });
    await page.locator('[data-glade-debug-output="issues"]').waitFor({ state: "attached", timeout: 60000 });

    await page.evaluate(() => {
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
      return {
        mode: console?.dataset.gladeWorkbenchMode,
        columns: getComputedStyle(console).gridTemplateColumns.split(" ").length,
        rows: getComputedStyle(console).gridTemplateRows.split(" ").length,
        sidebarWidth: box("[data-glade-workbench-sidebar]").width,
        canvasWidth: box("[data-glade-preview-canvas]").width,
        contextWidth: box("[data-glade-context-inspector]").width,
        dockHeight: box("[data-glade-debug-dock]").height,
        horizontalOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      };
    });
    assert.equal(consoleMetrics.mode, "recordPage");
    assert.equal(consoleMetrics.horizontalOverflow, false);
    assert.equal(consoleMetrics.columns, 3);
    assert.equal(consoleMetrics.rows, 2);
    assert.ok(consoleMetrics.sidebarWidth >= 260 && consoleMetrics.sidebarWidth <= 340);
    assert.ok(consoleMetrics.canvasWidth > consoleMetrics.sidebarWidth);
    assert.ok(consoleMetrics.canvasWidth > consoleMetrics.contextWidth);
    assert.ok(consoleMetrics.dockHeight >= 180 && consoleMetrics.dockHeight <= 300);

    await page.setViewportSize({ width: 900, height: 900 });
    const compactConsoleMetrics = await page.evaluate(() => {
      const box = (selector) => document.querySelector(selector).getBoundingClientRect();
      const console = document.querySelector("[data-glade-workbench-console]");
      return {
        columns: getComputedStyle(console).gridTemplateColumns.split(" ").length,
        rows: getComputedStyle(console).gridTemplateRows.split(" ").length,
        sidebarTop: box("[data-glade-workbench-sidebar]").top,
        canvasTop: box("[data-glade-preview-canvas]").top,
        contextTop: box("[data-glade-context-inspector]").top,
        dockTop: box("[data-glade-debug-dock]").top,
        horizontalOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      };
    });
    assert.equal(compactConsoleMetrics.horizontalOverflow, false);
    assert.equal(compactConsoleMetrics.columns, 1);
    assert.equal(compactConsoleMetrics.rows, 4);
    assert.ok(compactConsoleMetrics.sidebarTop < compactConsoleMetrics.canvasTop);
    assert.ok(compactConsoleMetrics.canvasTop < compactConsoleMetrics.contextTop);
    assert.ok(compactConsoleMetrics.contextTop < compactConsoleMetrics.dockTop);
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
        outsideContextDisplay: getComputedStyle(document.querySelector(".glade-context-panel")).display,
        layoutColumns: getComputedStyle(document.querySelector(".glade-builder-layout")).gridTemplateColumns.split(" ").length,
        paletteWidth: box(".glade-builder-palette").width,
        canvasShellWidth: box(".glade-builder-canvas-shell").width,
        propertiesWidth: box(".glade-builder-properties").width,
        canvasColumns: getComputedStyle(document.querySelector(".glade-page-canvas")).gridTemplateColumns.split(" ").length,
        consoleControlDisplay: getComputedStyle(document.querySelector(".glade-checkbox-control")).display,
        consoleInputWidth: box(".glade-checkbox-control input").width,
        mediumButtonWidth: box('[data-glade-form-factor-option="Medium"]').width,
        routePickerCount: document.querySelectorAll("details.glade-route-picker").length,
        routeMenuOpen: document.querySelector("details.glade-route-menu")?.open ?? true,
        routeMenuTop: box(".glade-route-menu").top,
        builderTop: box("[data-glade-workbench-builder]").top,
        inertButtonCount: Array.from(document.querySelectorAll("button:disabled")).filter((button) =>
          /Analyze|Activation|Save|Cut|Copy|Paste|Fields|Help/.test(button.textContent),
        ).length,
        horizontalOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      };
    });

    assert.equal(metrics.horizontalOverflow, false);
    assert.equal(metrics.builderActive, "true");
    assert.equal(metrics.outsideContextDisplay, "none");
    assert.equal(metrics.routePickerCount, 0);
    assert.equal(metrics.routeMenuOpen, false);
    assert.ok(metrics.routeMenuTop <= metrics.builderTop + 12, `route menu top = ${metrics.routeMenuTop}, builder top = ${metrics.builderTop}`);
    assert.equal(metrics.layoutColumns, 3);
    assert.equal(metrics.inertButtonCount, 0);
    assert.ok(metrics.commandbarHeight <= 52, `commandbar height = ${metrics.commandbarHeight}`);
    assert.ok(metrics.paletteWidth <= 295, `palette width = ${metrics.paletteWidth}`);
    assert.ok(metrics.propertiesWidth <= 360, `properties width = ${metrics.propertiesWidth}`);
    assert.ok(metrics.canvasShellWidth > metrics.paletteWidth, `canvas ${metrics.canvasShellWidth} palette ${metrics.paletteWidth}`);
    assert.ok(metrics.canvasShellWidth > metrics.propertiesWidth, `canvas ${metrics.canvasShellWidth} properties ${metrics.propertiesWidth}`);
    assert.equal(metrics.canvasColumns, 2);
    assert.equal(metrics.consoleControlDisplay, "flex");
    assert.ok(metrics.consoleInputWidth <= 20, `console input width = ${metrics.consoleInputWidth}`);
    assert.ok(metrics.mediumButtonWidth >= 64, `medium button width = ${metrics.mediumButtonWidth}`);
    const noSidecar = await page.evaluate(() => ({
      mobileSidecarCount: document.querySelectorAll("[data-glade-mobile-sidecar], .glade-mobile-frame").length,
      previewCanvasCount: document.querySelectorAll("[data-glade-preview-canvas]").length,
    }));
    assert.equal(noSidecar.mobileSidecarCount, 0);
    assert.equal(noSidecar.previewCanvasCount, 1);
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
        layoutColumns: getComputedStyle(document.querySelector(".glade-builder-layout")).gridTemplateColumns.split(" ").length,
        canvasColumns: getComputedStyle(document.querySelector(".glade-page-canvas")).gridTemplateColumns.split(" ").length,
        routePickerCount: document.querySelectorAll("details.glade-route-picker").length,
        routeMenuOpen: document.querySelector("details.glade-route-menu")?.open ?? true,
        horizontalOverflow: document.documentElement.scrollWidth > document.documentElement.clientWidth,
      };
    });

    assert.equal(metrics.horizontalOverflow, false);
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
    const targetOptions = await page.locator("[data-glade-target-picker] option").evaluateAll((options) =>
      options.map((option) => option.value),
    );
    assert.deepEqual(
      ["utilityBar", "flowScreen", "flowAction"].every((value) => targetOptions.includes(value)),
      true,
    );
    await page.locator("[data-glade-target-picker]").selectOption("recordPage");
    await page.locator("[data-glade-sample-record]").click();
    await page.locator("[data-glade-state-key]").fill("c__view");
    await page.locator("[data-glade-state-value]").fill("detail");
    await page.locator("[data-glade-community-selector]").fill("Partner_Portal");
    await page.locator("[data-glade-console-mode]").check();
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
  } finally {
    await browser.close();
    await server.close();
  }

  assert.deepEqual(consoleErrors, []);
  assert.deepEqual(pageErrors, []);
});
