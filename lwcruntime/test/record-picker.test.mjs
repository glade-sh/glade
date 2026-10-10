import assert from "node:assert/strict";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import { repoRoot, requireLWCToolchain } from "./helpers.mjs";
import { l19RuntimeImports, serveL19RuntimeModule } from "./l19-runtime-modules.mjs";

function serveModule(urlPath, res) {
  if (serveL19RuntimeModule(urlPath, res)) return;
  const routes = {
    "/lightning/vendor/lwc.js": path.join(repoRoot, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js"),
    "/lightning/vendor/synthetic-shadow.js": path.join(repoRoot, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js"),
    "/lightning/runtime/shell/diagnostics.js": path.join(repoRoot, "lwcruntime/src/shell/diagnostics.mjs"),
  };
  let filePath = routes[urlPath];
  if (!filePath && urlPath.startsWith("/lightning/shims/lightning/")) {
    const name = urlPath.slice("/lightning/shims/lightning/".length).replace(/\.(js|mjs)$/, "");
    filePath = path.join(repoRoot, "lwcruntime/src/lightning", `${name}.mjs`);
  }
  if (!filePath || !fs.existsSync(filePath)) {
    res.writeHead(404);
    res.end("missing " + urlPath);
    return;
  }
  res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
  res.end(fs.readFileSync(filePath));
}

function startRecordPickerServer({ preselected = true } = {}) {
  const calls = [];
  const recordCalls = [];
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, "http://localhost");
    if (url.pathname === "/lightning/wire/recordPickerSearch" && req.method === "POST") {
      let body = "";
      req.on("data", (chunk) => { body += String(chunk); });
      req.on("end", () => {
        const payload = JSON.parse(body || "{}");
        calls.push(payload);
        res.writeHead(200, { "Content-Type": "application/json; charset=utf-8" });
        res.end(JSON.stringify({ data: {
          objectApiName: "Account",
          records: [{ id: "001000000000001AAA", apiName: "Account", title: "Acme Lodge", fields: {
            Name: { value: "Acme Lodge", displayValue: "Acme Lodge" },
            Phone: { value: "907-555-0100", displayValue: "907-555-0100" },
          } }],
        } }));
      });
      return;
    }
    if (url.pathname === "/lightning/wire/getRecord" && req.method === "POST") {
      let body = "";
      req.on("data", (chunk) => {
        body += String(chunk);
      });
      req.on("end", () => {
        const payload = JSON.parse(body || "{}");
        recordCalls.push(payload);
        res.writeHead(200, { "Content-Type": "application/json; charset=utf-8" });
        res.end(JSON.stringify({
          data: {
            id: "001000000000001AAA",
            apiName: "Account",
            fields: {
              Name: { value: "Acme Lodge", displayValue: "Acme Lodge" },
              Phone: { value: "907-555-0100", displayValue: "907-555-0100" },
            },
          },
        }));
      });
      return;
    }
    if (url.pathname === "/record-picker.html") {
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(`<!DOCTYPE html><html><head>
  <script>window.process = { env: { NODE_ENV: "production" } };</script>
  <script type="importmap">${JSON.stringify({ imports: {
    "lwc": "/lightning/vendor/lwc.js",
    "@lwc/synthetic-shadow": "/lightning/vendor/synthetic-shadow.js",
    "@glade/shell/diagnostics": "/lightning/runtime/shell/diagnostics.js",
    ...l19RuntimeImports,
    "lightning/recordPicker": "/lightning/shims/lightning/recordPicker.js"
  } })}</script>
</head><body><div id="host"></div><script type="module" src="/record-picker-entry.js"></script></body></html>`);
      return;
    }
    if (url.pathname === "/record-picker-entry.js") {
      res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
      res.end(`
import "@lwc/synthetic-shadow";
import { createElement } from "lwc";
import RecordPicker from "lightning/recordPicker";
const picker = createElement("lightning-record-picker", { is: RecordPicker });
Object.assign(picker, {
  label: "Account",
  objectApiName: "Account",
  value: ${JSON.stringify(preselected ? "001000000000001AAA" : "")},
  placeholder: "Find an account",
  matchingInfo: { primaryField: { fieldPath: "Name" } },
  displayInfo: { primaryField: "Name", additionalFields: ["Phone"] },
});
picker.addEventListener("change", (event) => { window.__recordPickerChange = event.detail; });
document.getElementById("host").appendChild(picker);
`);
      return;
    }
    serveModule(url.pathname, res);
  });
  return new Promise((resolve) => {
    server.listen(0, "127.0.0.1", () => {
      resolve({
        baseURL: `http://127.0.0.1:${server.address().port}`,
        calls,
        recordCalls,
        close: () => new Promise((r) => server.close(r)),
      });
    });
  });
}

// Native r_picker_preselected (59/67) backs the hydrated public control.
test("record picker hydrates a preselected record through uiRecordApi", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const server = await startRecordPickerServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    const pageErrors = [];
    const consoleErrors = [];
    page.on("pageerror", (err) => pageErrors.push(err.message));
    page.on("console", (msg) => {
      if (msg.type() === "error") {
        consoleErrors.push(msg.text());
      }
    });
    await page.goto(`${server.baseURL}/record-picker.html`, { waitUntil: "networkidle" });
    // r_picker_preselected observes the hydrated public control. Private
    // component fields are not properties of the custom-element host.
    await page.waitForFunction(() => document.querySelector("lightning-record-picker")?.shadowRoot?.querySelector("input")?.value === "Acme Lodge");
    assert.equal(await page.locator("lightning-record-picker input").inputValue(), "Acme Lodge");
    assert.equal(await page.locator("lightning-record-picker input").evaluate((input) => input.readOnly), true);
    assert.equal(await page.locator("lightning-record-picker").evaluate((picker) => picker.value), "001000000000001AAA");
    assert.equal(server.recordCalls[0].recordId, "001000000000001AAA");
    assert.deepEqual(server.recordCalls[0].fields, ["Account.Name", "Account.Phone"]);
    assert.equal(await page.evaluate(() => window.__recordPickerChange), undefined);
    assert.deepEqual(pageErrors, []);
    assert.deepEqual(consoleErrors, []);
  } finally {
    await browser.close();
    await server.close();
  }
});

// Local request/selection regression coverage. The native search observer's
// malformed injected change is retained as a boundary, not successful parity.
test("record picker searches and selects after committing ordinary input text", async (t) => {
  if (!requireLWCToolchain(t)) return;
  const server = await startRecordPickerServer({ preselected: false });
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    const pageErrors = [], consoleErrors = [];
    page.on("pageerror", (error) => pageErrors.push(error.message));
    page.on("console", (message) => {
      if (message.type() === "error") consoleErrors.push(message.text());
    });
    await page.goto(`${server.baseURL}/record-picker.html`, { waitUntil: "networkidle" });
    const input = page.locator("lightning-record-picker input");
    await input.fill("Ac");
    const suggestion = page.locator("lightning-record-picker button", { hasText: "Acme Lodge" });
    await suggestion.waitFor({ timeout: 10000 });
    assert.deepEqual(server.calls[0], {
      objectApiName: "Account", searchTerm: "Ac", fields: ["Name", "Phone"],
      matchingFields: ["Name"], pageSize: 10,
    });
    assert.match(await page.locator("lightning-record-picker").innerText(), /907-555-0100/);
    await input.evaluate((element) => element.dispatchEvent(new Event("change", { bubbles: true, composed: true })));
    assert.equal(await input.inputValue(), "Ac");
    assert.equal(await suggestion.count(), 1);
    assert.equal(await page.evaluate(() => window.__recordPickerChange), undefined);
    assert.equal(server.calls.length, 1);
    await suggestion.click();
    await page.waitForFunction(() => window.__recordPickerChange?.recordId === "001000000000001AAA");
    assert.deepEqual(await page.evaluate(() => window.__recordPickerChange), {
      recordId: "001000000000001AAA", value: "001000000000001AAA",
    });
    assert.equal(await page.locator("lightning-record-picker").evaluate((picker) => picker.value), "001000000000001AAA");
    await page.waitForFunction(() => document.querySelector("lightning-record-picker")?.shadowRoot?.querySelector("input")?.value === "Acme Lodge");
    assert.equal(await input.evaluate((element) => element.readOnly), true);
    assert.deepEqual(pageErrors, []);
    assert.deepEqual(consoleErrors, []);
  } finally {
    await browser.close();
    await server.close();
  }
});
