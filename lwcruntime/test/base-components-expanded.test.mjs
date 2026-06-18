import assert from "node:assert/strict";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import { repoRoot, requireLWCToolchain } from "./helpers.mjs";

const expandedImports = {
  "lwc": "/lightning/vendor/lwc.js",
  "@lwc/synthetic-shadow": "/lightning/vendor/synthetic-shadow.js",
  "@glade/slds": "/lightning/runtime/slds/slds-loader.js",
  "@glade/shell/diagnostics": "/lightning/runtime/shell/diagnostics.js",
  "lightning/breadcrumb": "/lightning/shims/lightning/breadcrumb.js",
  "lightning/breadcrumbs": "/lightning/shims/lightning/breadcrumbs.js",
  "lightning/carousel": "/lightning/shims/lightning/carousel.js",
  "lightning/carouselImage": "/lightning/shims/lightning/carouselImage.js",
  "lightning/checkboxGroup": "/lightning/shims/lightning/checkboxGroup.js",
  "lightning/dualListbox": "/lightning/shims/lightning/dualListbox.js",
  "lightning/fileUpload": "/lightning/shims/lightning/fileUpload.js",
  "lightning/formattedEmail": "/lightning/shims/lightning/formattedEmail.js",
  "lightning/inputRichText": "/lightning/shims/lightning/inputRichText.js",
  "lightning/map": "/lightning/shims/lightning/map.js",
  "lightning/menuDivider": "/lightning/shims/lightning/menuDivider.js",
  "lightning/progressBar": "/lightning/shims/lightning/progressBar.js",
  "lightning/progressRing": "/lightning/shims/lightning/progressRing.js",
  "lightning/quickActionPanel": "/lightning/shims/lightning/quickActionPanel.js",
  "lightning/recordPicker": "/lightning/shims/lightning/recordPicker.js",
  "lightning/select": "/lightning/shims/lightning/select.js",
  "lightning/slider": "/lightning/shims/lightning/slider.js",
  "lightning/tile": "/lightning/shims/lightning/tile.js",
  "lightning/treeGrid": "/lightning/shims/lightning/treeGrid.js",
};

function serveRuntimeFile(urlPath, res) {
  const routes = {
    "/lightning/vendor/lwc.js": path.join(repoRoot, "third_party/lwc/node_modules/@lwc/engine-dom/dist/index.js"),
    "/lightning/vendor/synthetic-shadow.js": path.join(repoRoot, "third_party/lwc/node_modules/@lwc/synthetic-shadow/dist/index.js"),
    "/lightning/runtime/slds/slds-loader.js": path.join(repoRoot, "lwcruntime/src/slds/slds-loader.mjs"),
    "/lightning/runtime/shell/diagnostics.js": path.join(repoRoot, "lwcruntime/src/shell/diagnostics.mjs"),
  };
  let filePath = routes[urlPath];
  if (!filePath && urlPath.startsWith("/lightning/runtime/slds/")) {
    filePath = path.normalize(path.join(repoRoot, "lwcruntime/src/slds", urlPath.slice("/lightning/runtime/slds/".length)));
  }
  if (!filePath && urlPath.startsWith("/lightning/shims/lightning/")) {
    const name = urlPath.slice("/lightning/shims/lightning/".length).replace(/\.(js|mjs)$/, "");
    filePath = path.join(repoRoot, "lwcruntime/src/lightning", `${name}.mjs`);
  }
  if (!filePath || !fs.existsSync(filePath)) {
    res.writeHead(404);
    res.end("missing " + urlPath);
    return;
  }
  const contentType = filePath.endsWith(".css") ? "text/css; charset=utf-8" : "application/javascript; charset=utf-8";
  res.writeHead(200, { "Content-Type": contentType });
  res.end(fs.readFileSync(filePath));
}

function startExpandedBaseComponentServer() {
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, "http://localhost");
    if (url.pathname === "/expanded.html") {
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(`<!DOCTYPE html><html><head>
  <script>window.process = { env: { NODE_ENV: "production" } };</script>
  <script type="importmap">${JSON.stringify({ imports: expandedImports })}</script>
</head><body><div id="host"></div><script type="module" src="/expanded-entry.js"></script></body></html>`);
      return;
    }
    if (url.pathname === "/expanded-entry.js") {
      res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
      res.end(`
import "@lwc/synthetic-shadow";
import { createElement } from "lwc";
import { loadSLDS } from "@glade/slds";
import Breadcrumb from "lightning/breadcrumb";
import Breadcrumbs from "lightning/breadcrumbs";
import Carousel from "lightning/carousel";
import CarouselImage from "lightning/carouselImage";
import CheckboxGroup from "lightning/checkboxGroup";
import DualListbox from "lightning/dualListbox";
import FileUpload from "lightning/fileUpload";
import FormattedEmail from "lightning/formattedEmail";
import InputRichText from "lightning/inputRichText";
import Map from "lightning/map";
import MenuDivider from "lightning/menuDivider";
import ProgressBar from "lightning/progressBar";
import ProgressRing from "lightning/progressRing";
import QuickActionPanel from "lightning/quickActionPanel";
import RecordPicker from "lightning/recordPicker";
import Select from "lightning/select";
import Slider from "lightning/slider";
import Tile from "lightning/tile";
import TreeGrid from "lightning/treeGrid";
const host = document.getElementById("host");
function append(tag, Ctor, props = {}) {
  const el = createElement(tag, { is: Ctor });
  Object.assign(el, props);
  host.appendChild(el);
  return el;
}
await loadSLDS();
append("lightning-formatted-email", FormattedEmail, { value: "trail@example.com", label: "Trail Email" });
const checks = append("lightning-checkbox-group", CheckboxGroup, {
  label: "Checks",
  value: ["alpha"],
  options: [{ label: "Alpha", value: "alpha" }, { label: "Beta", value: "beta" }]
});
checks.addEventListener("change", (event) => { window.__checkboxGroup = event.detail; });
const dual = append("lightning-dual-listbox", DualListbox, {
  label: "Providers",
  sourceLabel: "Available",
  selectedLabel: "Selected",
  value: ["alpha"],
  options: [{ label: "Alpha", value: "alpha" }, { label: "Beta", value: "beta" }]
});
dual.addEventListener("change", (event) => { window.__dualListbox = event.detail; });
const select = append("lightning-select", Select, {
  label: "Status",
  value: "open",
  options: [{ label: "Open", value: "open" }, { label: "Closed", value: "closed" }]
});
select.addEventListener("change", (event) => { window.__select = event.detail; });
const slider = append("lightning-slider", Slider, { label: "Confidence", value: "20", min: "0", max: "100", step: "5" });
slider.addEventListener("change", (event) => { window.__slider = event.detail; });
const richText = append("lightning-input-rich-text", InputRichText, { label: "Summary", value: "<p>Initial rich text</p>" });
richText.addEventListener("change", (event) => { window.__richText = event.detail; });
const menuDivider = append("lightning-menu-divider", MenuDivider);
append("lightning-progress-bar", ProgressBar, { value: "65", size: "medium" });
append("lightning-progress-ring", ProgressRing, { value: "80", variant: "base" });
append("lightning-tile", Tile, { label: "Provider Tile", href: "/lightning/r/Account/001000000000001AAA/view" });
const breadcrumbs = append("lightning-breadcrumbs", Breadcrumbs);
const breadcrumb = createElement("lightning-breadcrumb", { is: Breadcrumb });
breadcrumb.label = "Account";
breadcrumb.href = "/lightning/o/Account/home";
breadcrumb.addEventListener("active", (event) => { window.__breadcrumb = event.detail; });
breadcrumbs.appendChild(breadcrumb);
append("lightning-tree-grid", TreeGrid, {
  keyField: "id",
  columns: [{ label: "Name", fieldName: "name" }],
  data: [{ id: "1", name: "Root Provider", _children: [{ id: "2", name: "Child Provider" }] }]
});
append("lightning-map", Map, {
  mapMarkers: [{ title: "Twin Lakes", location: { City: "Twin Lakes", State: "AK" } }]
});
const carousel = append("lightning-carousel", Carousel);
const image = createElement("lightning-carousel-image", { is: CarouselImage });
image.src = "data:image/gif;base64,R0lGODlhAQABAAAAACw=";
image.header = "Trail Image";
image.description = "Rendered image";
carousel.appendChild(image);
const panel = append("lightning-quick-action-panel", QuickActionPanel, { header: "Expanded Action" });
const footer = document.createElement("button");
footer.slot = "footer";
footer.textContent = "Done";
panel.appendChild(footer);
const upload = append("lightning-file-upload", FileUpload, { label: "Upload File", accept: ".txt" });
upload.addEventListener("uploadfinished", (event) => { window.__upload = event.detail; });
const picker = append("lightning-record-picker", RecordPicker, { label: "Account", value: "001000000000001AAA" });
picker.addEventListener("change", (event) => { window.__recordPicker = event.detail; });
window.__menuDividerTag = menuDivider.tagName;
`);
      return;
    }
    serveRuntimeFile(url.pathname, res);
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

test("expanded phase 3 base components render and dispatch local events", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const server = await startExpandedBaseComponentServer();
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
    await page.goto(`${server.baseURL}/expanded.html`, { waitUntil: "networkidle" });
    assert.equal(await page.locator("lightning-formatted-email a").getAttribute("href"), "mailto:trail@example.com");
    await page.locator("lightning-checkbox-group label", { hasText: "Beta" }).click();
    assert.deepEqual(await page.evaluate(() => window.__checkboxGroup), { value: ["alpha", "beta"] });
    assert.deepEqual(await page.locator('lightning-dual-listbox select[data-list="source"] option').allTextContents(), ["Beta"]);
    assert.deepEqual(await page.locator('lightning-dual-listbox select[data-list="selected"] option').allTextContents(), ["Alpha"]);
    await page.locator('lightning-dual-listbox select[data-list="source"]').selectOption("beta");
    await page.getByRole("button", { name: "Move selection to Selected" }).click();
    assert.deepEqual(await page.evaluate(() => window.__dualListbox), { value: ["alpha", "beta"] });
    await page.locator("lightning-select select").selectOption("closed");
    assert.deepEqual(await page.evaluate(() => window.__select), { value: "closed", checked: false });
    await page.locator("lightning-slider input").evaluate((input) => {
      input.value = "45";
      input.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
      input.dispatchEvent(new Event("change", { bubbles: true, composed: true }));
    });
    await page.waitForFunction(() => window.__slider);
    assert.deepEqual(await page.evaluate(() => window.__slider), { value: "45", checked: false });
    await page.locator("lightning-input-rich-text textarea").fill("<p>Changed</p>");
    await page.locator("lightning-input-rich-text textarea").dispatchEvent("change");
    assert.deepEqual(await page.evaluate(() => window.__richText), { value: "<p>Changed</p>" });
    assert.equal(await page.locator('lightning-menu-divider [role="separator"]').count(), 1);
    assert.equal(await page.locator("lightning-progress-bar [role='progressbar']").getAttribute("aria-valuenow"), "65");
    assert.match(await page.locator("lightning-progress-ring").innerText(), /80/);
    assert.match(await page.locator("lightning-tile").innerText(), /Provider Tile/);
    assert.match(await page.locator("lightning-breadcrumbs").innerText(), /Account/);
    await page.locator("lightning-breadcrumb a").click();
    assert.deepEqual(await page.evaluate(() => window.__breadcrumb), { value: "Account", label: "Account" });
    assert.match(await page.locator("lightning-tree-grid").innerText(), /Child Provider/);
    assert.match(await page.locator("lightning-map").innerText(), /Twin Lakes/);
    assert.match(await page.locator("lightning-carousel").innerText(), /Trail Image/);
    assert.match(await page.locator("lightning-quick-action-panel").innerText(), /Expanded Action/);
    await page.locator("lightning-file-upload input").setInputFiles({
      name: "phase3.txt",
      mimeType: "text/plain",
      buffer: Buffer.from("phase 3"),
    });
    assert.deepEqual(await page.evaluate(() => window.__upload), {
      files: [{ name: "phase3.txt", documentId: "069000000000001AAA" }],
    });
    await page.locator("lightning-record-picker input").evaluate((input) => {
      input.value = "001000000000003AAA";
      input.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
    });
    assert.deepEqual(await page.evaluate(() => window.__recordPicker), {
      recordId: "001000000000003AAA",
      value: "001000000000003AAA",
    });
    assert.deepEqual(pageErrors, []);
    assert.deepEqual(consoleErrors, []);
  } finally {
    await browser.close();
    await server.close();
  }
});
