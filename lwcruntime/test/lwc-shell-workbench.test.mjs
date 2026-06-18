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
  "/lightning/runtime/shell/toast-service.js": "lwcruntime/src/shell/toast-service.mjs",
  "/lightning/runtime/shell/toast-service.mjs": "lwcruntime/src/shell/toast-service.mjs",
  "/lightning/runtime/slds/slds-loader.js": "lwcruntime/src/slds/slds-loader.mjs",
  "/lightning/runtime/slds/slds-loader.mjs": "lwcruntime/src/slds/slds-loader.mjs",
  "/lightning/runtime/shims/community.js": "lwcruntime/src/shims/community.mjs",
  "/lightning/runtime/shims/community.mjs": "lwcruntime/src/shims/community.mjs",
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
  <main data-glade-main>Mounted component region</main>
  <section data-glade-flow-events></section>
  <aside data-glade-context-panel></aside>
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
    assert.ok(await page.locator('a[data-glade-route][href^="/lwc/preview/record/Account/"]').count());
    assert.ok(await page.locator('a[data-glade-route][href="/lwc/preview/app/Sales_Dashboard"]').count());
    assert.ok(await page.locator('a[data-glade-route][href="/lwc/preview/tab/Lwc_Probe"]').count());
    assert.ok(await page.locator('a[data-glade-route][href="/lwc/preview/community/Partner_Portal/Account"]').count());

    const workbench = await page.locator("#glade-lwc-workbench").textContent();
    const model = JSON.parse(workbench || "{}");
    assert.equal(model.title, "Glade Lightning Shell");
    assert.ok(model.routes.some((route) => route.url === "/lwc/preview/app/Sales_Dashboard"));
    assert.ok(model.routes.some((route) => route.url === "/lwc/preview/tab/Lwc_Probe"));
    assert.ok(model.routes.some((route) => route.url === "/lwc/preview/community/Partner_Portal/Account"));

    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`, {
      waitUntil: "networkidle",
    });
    assert.match(await page.locator("c-record-probe").innerText({ timeout: 60000 }), /Local Shell Account/);
    await assert.match(await page.locator("[data-glade-context-panel]").innerText(), /Record/);
    await assert.match(await page.locator("[data-glade-context-panel]").innerText(), /001000000000001AAA/);
    assert.equal(await page.locator("script#glade-lwc-context").evaluate((node) => JSON.parse(node.textContent).kind), "recordPage");

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

    await page.locator("[data-glade-workbench-builder]").waitFor({ timeout: 60000 });
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
    assert.match(text, /LOCAL SHELL CONTEXT/);
    assert.match(text, /001000000000001AAA/);
    const contextPanel = await page.locator("[data-glade-context-panel]").innerText({ timeout: 60000 });
    assert.match(contextPanel, /standard__recordPage/);
    assert.match(contextPanel, /Account/);
    assert.match(contextPanel, /001000000000001AAA/);
    assert.equal(await page.locator(".glade-route-picker[open]").count(), 0);
    assert.equal(await page.locator(".glade-stage > .glade-region").count(), 0);

    const model = JSON.parse(await page.locator("#glade-lwc-workbench").textContent());
    assert.ok(model.components.some((component) => component.qualifiedName === "c:contextProbe"));
    assert.equal(await page.locator('[data-glade-page-canvas] [data-glade-region-drop="main"]').count(), 1);
  } finally {
    await browser.close();
    await server.close();
  }

  assert.deepEqual(consoleErrors, []);
  assert.deepEqual(pageErrors, []);
});
