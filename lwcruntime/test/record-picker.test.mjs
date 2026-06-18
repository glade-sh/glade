import assert from "node:assert/strict";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import { repoRoot, requireLWCToolchain } from "./helpers.mjs";

function serveModule(urlPath, res) {
  const routes = {
    "/lightning/vendor/lwc.js": path.join(repoRoot, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js"),
    "/lightning/vendor/synthetic-shadow.js": path.join(repoRoot, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js"),
    "/lightning/runtime/shell/diagnostics.js": path.join(repoRoot, "lwcruntime/src/shell/diagnostics.mjs"),
    "/lightning/shims/lightning/uiRecordApi.js": null,
  };
  if (urlPath === "/lightning/shims/lightning/uiRecordApi.js") {
    res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
    res.end(`export async function __gladeRecordPickerSearch(config = {}) {
  const response = await fetch("/lightning/wire/recordPickerSearch", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(config),
  });
  const payload = await response.json();
  if (payload.error) {
    throw new Error(payload.error.message || "record picker search failed");
  }
  return payload.data;
}
`);
    return;
  }
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

function startRecordPickerServer() {
  const calls = [];
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, "http://localhost");
    if (url.pathname === "/lightning/wire/recordPickerSearch" && req.method === "POST") {
      let body = "";
      req.on("data", (chunk) => {
        body += String(chunk);
      });
      req.on("end", () => {
        const payload = JSON.parse(body || "{}");
        calls.push(payload);
        res.writeHead(200, { "Content-Type": "application/json; charset=utf-8" });
        res.end(JSON.stringify({
          data: {
            objectApiName: "Account",
            records: [{
              id: "001000000000001AAA",
              apiName: "Account",
              title: "Acme Lodge",
              fields: {
                Name: { value: "Acme Lodge", displayValue: "Acme Lodge" },
                Phone: { value: "907-555-0100", displayValue: "907-555-0100" },
              },
            }],
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
    "lightning/uiRecordApi": "/lightning/shims/lightning/uiRecordApi.js",
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
        close: () => new Promise((r) => server.close(r)),
      });
    });
  });
}

test("record picker searches records through uiRecordApi helper", async (t) => {
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
    await page.locator("lightning-record-picker input").fill("Ac");
    await page.locator("lightning-record-picker button", { hasText: "Acme Lodge" }).waitFor({
      timeout: 10000,
    }).catch(async (err) => {
      const text = await page.locator("lightning-record-picker").innerText().catch(() => "");
      const state = await page.locator("lightning-record-picker").evaluate((el) => ({
        records: el.recordPickerRecords,
        searching: el.searching,
        errorMessage: el.errorMessage,
        shadow: el.shadowRoot?.innerHTML,
      })).catch((stateErr) => ({ stateError: stateErr.message }));
      throw new Error(`${err.message}; calls=${JSON.stringify(server.calls)} text=${JSON.stringify(text)} state=${JSON.stringify(state)} pageErrors=${JSON.stringify(pageErrors)} consoleErrors=${JSON.stringify(consoleErrors)}`);
    });
    assert.deepEqual(server.calls[0], {
      objectApiName: "Account",
      searchTerm: "Ac",
      fields: ["Name", "Phone"],
      matchingFields: ["Name"],
      pageSize: 10,
    });
    assert.match(await page.locator("lightning-record-picker").innerText(), /907-555-0100/);
    await page.locator("lightning-record-picker button", { hasText: "Acme Lodge" }).click();
    await page.waitForFunction(() => window.__recordPickerChange, null, { timeout: 5000 }).catch(async (err) => {
      const state = await page.locator("lightning-record-picker").evaluate((el) => ({
        value: el.value,
        searchTerm: el.searchTerm,
        records: el.recordPickerRecords,
        shadow: el.shadowRoot?.innerHTML,
      })).catch((stateErr) => ({ stateError: stateErr.message }));
      throw new Error(`${err.message}; state=${JSON.stringify(state)} pageErrors=${JSON.stringify(pageErrors)} consoleErrors=${JSON.stringify(consoleErrors)}`);
    });
    assert.deepEqual(await page.evaluate(() => window.__recordPickerChange), {
      recordId: "001000000000001AAA",
      value: "001000000000001AAA",
    });
    assert.deepEqual(pageErrors, []);
    assert.deepEqual(consoleErrors, []);
  } finally {
    await browser.close();
    await server.close();
  }
});
