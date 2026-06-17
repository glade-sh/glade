import assert from "node:assert/strict";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import { repoRoot, startLWCDevServer } from "./helpers.mjs";

const shellFiles = {
  "/lightning/runtime/shell/app.js": "lwcruntime/src/shell/app.mjs",
  "/lightning/runtime/shell/app.mjs": "lwcruntime/src/shell/app.mjs",
  "/lightning/runtime/shell/router.js": "lwcruntime/src/shell/router.mjs",
  "/lightning/runtime/shell/router.mjs": "lwcruntime/src/shell/router.mjs",
  "/lightning/runtime/shell/context-panel.js": "lwcruntime/src/shell/context-panel.mjs",
  "/lightning/runtime/shell/context-panel.mjs": "lwcruntime/src/shell/context-panel.mjs",
  "/lightning/runtime/shell/diagnostics.js": "lwcruntime/src/shell/diagnostics.mjs",
  "/lightning/runtime/shell/diagnostics.mjs": "lwcruntime/src/shell/diagnostics.mjs",
  "/lightning/runtime/shell/navigation-service.js": "lwcruntime/src/shell/navigation-service.mjs",
  "/lightning/runtime/shell/navigation-service.mjs": "lwcruntime/src/shell/navigation-service.mjs",
  "/lightning/runtime/shell/toast-service.js": "lwcruntime/src/shell/toast-service.mjs",
  "/lightning/runtime/shell/toast-service.mjs": "lwcruntime/src/shell/toast-service.mjs",
  "/lightning/runtime/slds/slds-loader.js": "lwcruntime/src/slds/slds-loader.mjs",
  "/lightning/runtime/slds/slds-loader.mjs": "lwcruntime/src/slds/slds-loader.mjs",
  "/lightning/runtime/slds/glade-slds.css": "lwcruntime/src/slds/glade-slds.css",
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
  <aside data-glade-context-panel></aside>
  <script type="module">
    import { bootGladeShell } from "/lightning/runtime/shell/app.js";
    import { reportDiagnostic } from "/lightning/runtime/shell/diagnostics.js";
    window.__boot = await bootGladeShell();
    reportDiagnostic({ code: "GLADELWC999", message: "probe diagnostic" });
  </script>
</body>
</html>`);
      return;
    }
    const normalizedPath = url.pathname.replace(/\.mjs$/, ".js");
    const file = shellFiles[normalizedPath];
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
      diagnostics: window.__gladeDiagnostics,
    }));

    assert.equal(result.routeKind, "record");
    assert.match(result.contextText, /standard__recordPage/);
    assert.match(result.contextText, /Account/);
    assert.match(result.contextText, /001000000000001AAA/);
    assert.match(result.contextText, /GLADELWC999: probe diagnostic/);
    assert.equal(result.sldsHref, "/lightning/runtime/slds/glade-slds.css");
    assert.equal(result.toastRegion, true);
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

    const workbench = await page.locator("#glade-lwc-workbench").textContent();
    const model = JSON.parse(workbench || "{}");
    assert.equal(model.title, "Glade Lightning Shell");
    assert.ok(model.routes.some((route) => route.url === "/lwc/preview/app/Sales_Dashboard"));
    assert.ok(model.routes.some((route) => route.url === "/lwc/preview/tab/Lwc_Probe"));

    await page.goto(`${server.baseURL}/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`, {
      waitUntil: "networkidle",
    });
    assert.match(await page.locator("c-record-probe").innerText({ timeout: 30000 }), /Local Shell Account/);
    await assert.match(await page.locator("[data-glade-context-panel]").innerText(), /Record/);
    await assert.match(await page.locator("[data-glade-context-panel]").innerText(), /001000000000001AAA/);
    assert.equal(await page.locator("script#glade-lwc-context").evaluate((node) => JSON.parse(node.textContent).kind), "recordPage");

    const contextResponse = await page.request.get(`${server.baseURL}/lightning/local/context.json?url=/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page`);
    assert.equal(contextResponse.ok(), true);
    const context = await contextResponse.json();
    assert.equal(context.pageReference.type, "standard__recordPage");
    assert.equal(context.context.recordId, "001000000000001AAA");
    assert.ok(context.mounts.some((mount) => mount.qualified === "c:recordProbe"));
  } finally {
    await browser.close();
    await server.close();
  }

  assert.deepEqual(consoleErrors, []);
  assert.deepEqual(pageErrors, []);
});
