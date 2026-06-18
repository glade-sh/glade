import assert from "node:assert/strict";
import fs from "node:fs";
import http from "node:http";
import path from "node:path";
import test from "node:test";
import { chromium } from "playwright";
import { defaultSLDSHref, repoRoot, requireLWCToolchain } from "./helpers.mjs";

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

function startBaseComponentServer() {
  const server = http.createServer((req, res) => {
    const url = new URL(req.url, "http://localhost");
    if (url.pathname === "/lightning/wire/getRecord" && req.method === "POST") {
      req.resume();
      res.writeHead(200, { "Content-Type": "application/json; charset=utf-8" });
      res.end(JSON.stringify({
        data: {
          id: "001000000000001AAA",
          apiName: "Account",
          fields: {
            Name: { value: "Local Shell Account", displayValue: "Local Shell Account" },
            Phone: { value: "415-555-0100", displayValue: "415-555-0100" },
          },
        },
      }));
      return;
    }
    if (url.pathname === "/lightning/wire/updateRecord" && req.method === "POST") {
      let body = "";
      req.on("data", (chunk) => {
        body += String(chunk);
      });
      req.on("end", () => {
        const payload = JSON.parse(body || "{}");
        res.writeHead(200, { "Content-Type": "application/json; charset=utf-8" });
        res.end(JSON.stringify({
          data: {
            id: payload.fields?.Id || "001000000000001AAA",
            apiName: "Account",
            fields: {
              Name: { value: payload.fields?.Name || "Updated Local Account", displayValue: payload.fields?.Name || "Updated Local Account" },
            },
          },
        }));
      });
      return;
    }
    if (url.pathname === "/lightning/wire/createRecord" && req.method === "POST") {
      let body = "";
      req.on("data", (chunk) => {
        body += String(chunk);
      });
      req.on("end", () => {
        const payload = JSON.parse(body || "{}");
        res.writeHead(200, { "Content-Type": "application/json; charset=utf-8" });
        res.end(JSON.stringify({
          data: {
            id: "001000000000002AAA",
            apiName: payload.apiName || payload.objectApiName || "Account",
            fields: Object.fromEntries(Object.entries(payload.fields || {}).map(([name, value]) => [name, { value, displayValue: String(value) }])),
          },
        }));
      });
      return;
    }
    if (url.pathname === "/lightning/wire/getRecordCreateDefaults" && req.method === "POST") {
      req.resume();
      res.writeHead(200, { "Content-Type": "application/json; charset=utf-8" });
      res.end(JSON.stringify({
        data: {
          objectInfos: {
            Account: {
              apiName: "Account",
              fields: {
                Name: { apiName: "Name", dataType: "String", label: "Account Name", required: true },
                Type: {
                  apiName: "Type",
                  dataType: "Picklist",
                  label: "Type",
                  picklistValues: [
                    { label: "Customer", value: "Customer" },
                    { label: "Partner", value: "Partner" },
                  ],
                },
              },
            },
          },
          record: {
            apiName: "Account",
            fields: {
              Name: { value: "", displayValue: "" },
              Type: { value: "Customer", displayValue: "Customer" },
            },
          },
        },
      }));
      return;
    }
    if (url.pathname === "/base.html") {
      res.writeHead(200, { "Content-Type": "text/html; charset=utf-8" });
      res.end(`<!DOCTYPE html>
<html>
<head>
  <script>window.process = { env: { NODE_ENV: "production" } };</script>
  <script type="importmap">${JSON.stringify({ imports: {
    "lwc": "/lightning/vendor/lwc.js",
    "@lwc/synthetic-shadow": "/lightning/vendor/synthetic-shadow.js",
    "@glade/slds": "/lightning/runtime/slds/slds-loader.js",
    "@glade/shell/diagnostics": "/lightning/runtime/shell/diagnostics.js",
    "lightning/button": "/lightning/shims/lightning/button.js",
    "lightning/buttonIcon": "/lightning/shims/lightning/buttonIcon.js",
    "lightning/input": "/lightning/shims/lightning/input.js",
    "lightning/inputField": "/lightning/shims/lightning/inputField.js",
    "lightning/textarea": "/lightning/shims/lightning/textarea.js",
    "lightning/combobox": "/lightning/shims/lightning/combobox.js",
    "lightning/card": "/lightning/shims/lightning/card.js",
    "lightning/layout": "/lightning/shims/lightning/layout.js",
    "lightning/layoutItem": "/lightning/shims/lightning/layoutItem.js",
    "lightning/datatable": "/lightning/shims/lightning/datatable.js",
    "lightning/recordForm": "/lightning/shims/lightning/recordForm.js",
    "lightning/recordEditForm": "/lightning/shims/lightning/recordEditForm.js",
    "lightning/tabset": "/lightning/shims/lightning/tabset.js",
    "lightning/tab": "/lightning/shims/lightning/tab.js",
    "lightning/icon": "/lightning/shims/lightning/icon.js",
    "lightning/modal": "/lightning/shims/lightning/modal.js",
    "lightning/accordion": "/lightning/shims/lightning/accordion.js",
    "lightning/accordionSection": "/lightning/shims/lightning/accordionSection.js",
    "lightning/avatar": "/lightning/shims/lightning/avatar.js",
    "lightning/badge": "/lightning/shims/lightning/badge.js",
    "lightning/buttonMenu": "/lightning/shims/lightning/buttonMenu.js",
    "lightning/checkboxGroup": "/lightning/shims/lightning/checkboxGroup.js",
    "lightning/fileUpload": "/lightning/shims/lightning/fileUpload.js",
    "lightning/flow": "/lightning/shims/lightning/flow.js",
    "lightning/formattedDateTime": "/lightning/shims/lightning/formattedDateTime.js",
    "lightning/formattedNumber": "/lightning/shims/lightning/formattedNumber.js",
    "lightning/formattedRichText": "/lightning/shims/lightning/formattedRichText.js",
    "lightning/helptext": "/lightning/shims/lightning/helptext.js",
    "lightning/menuItem": "/lightning/shims/lightning/menuItem.js",
    "lightning/pill": "/lightning/shims/lightning/pill.js",
    "lightning/quickActionPanel": "/lightning/shims/lightning/quickActionPanel.js",
    "lightning/radioGroup": "/lightning/shims/lightning/radioGroup.js",
    "lightning/recordPicker": "/lightning/shims/lightning/recordPicker.js",
    "lightning/verticalNavigation": "/lightning/shims/lightning/verticalNavigation.js",
    "lightning/verticalNavigationItem": "/lightning/shims/lightning/verticalNavigationItem.js"
  } })}</script>
</head>
<body><div id="host"></div><script type="module" src="/test-entry.js"></script></body>
</html>`);
      return;
    }
    if (url.pathname === "/test-entry.js") {
      res.writeHead(200, { "Content-Type": "application/javascript; charset=utf-8" });
      res.end(`
import "@lwc/synthetic-shadow";
import { createElement } from "lwc";
import { loadSLDS } from "@glade/slds";
import { diagnostics } from "@glade/shell/diagnostics";
import Button from "lightning/button";
import ButtonIcon from "lightning/buttonIcon";
import Input from "lightning/input";
import InputField from "lightning/inputField";
import Textarea from "lightning/textarea";
import Combobox from "lightning/combobox";
import Card from "lightning/card";
import Layout from "lightning/layout";
import LayoutItem from "lightning/layoutItem";
import Datatable from "lightning/datatable";
import RecordForm from "lightning/recordForm";
import RecordEditForm from "lightning/recordEditForm";
import Tabset from "lightning/tabset";
import Tab from "lightning/tab";
import Icon from "lightning/icon";
import Modal from "lightning/modal";
import Accordion from "lightning/accordion";
import AccordionSection from "lightning/accordionSection";
import Avatar from "lightning/avatar";
import Badge from "lightning/badge";
import ButtonMenu from "lightning/buttonMenu";
import CheckboxGroup from "lightning/checkboxGroup";
import FileUpload from "lightning/fileUpload";
import Flow from "lightning/flow";
import FormattedDateTime from "lightning/formattedDateTime";
import FormattedNumber from "lightning/formattedNumber";
import FormattedRichText from "lightning/formattedRichText";
import Helptext from "lightning/helptext";
import MenuItem from "lightning/menuItem";
import Pill from "lightning/pill";
import QuickActionPanel from "lightning/quickActionPanel";
import RadioGroup from "lightning/radioGroup";
import RecordPicker from "lightning/recordPicker";
import VerticalNavigation from "lightning/verticalNavigation";
import VerticalNavigationItem from "lightning/verticalNavigationItem";
const host = document.getElementById("host");
function append(tag, Ctor, props = {}) {
  const el = createElement(tag, { is: Ctor });
  Object.assign(el, props);
  host.appendChild(el);
  return el;
}
await loadSLDS();
const button = append("lightning-button", Button, { label: "Save", disabled: false, variant: "brand", name: "saveButton", value: "save" });
button.addEventListener("click", () => {
  window.__baseClickCount = (window.__baseClickCount || 0) + 1;
});
append("lightning-button-icon", ButtonIcon, { iconName: "utility:add", alternativeText: "Add Account", variant: "border-filled", size: "small", name: "addAccount", value: "add" });
append("lightning-input", Input, { label: "Name", value: "Ada", disabled: false });
const inputField = append("lightning-input-field", InputField, { fieldName: "Name", value: "Ada" });
inputField.setErrors({ message: "Required" });
inputField.wireRecordUi({ record: { id: "001000000000001AAA" } });
inputField.wirePicklistValues({ Name: { values: [{ label: "Warm", value: "Warm" }] } });
inputField.setValue("Grace");
const inputFieldBeforeClean = {
  errors: inputField.getErrors(),
  wiredData: inputField.getWiredData(),
  wiredPicklistValues: inputField.getWiredPicklistValues(),
  value: inputField.value,
  dirty: inputField.dirty,
};
inputField.clean();
inputField.setValue("Reset Candidate");
inputField.reset();
window.__baseInputFieldContracts = {
  ...inputFieldBeforeClean,
  dirtyAfterClean: inputField.dirty,
  resetValue: inputField.value,
  dirtyAfterReset: inputField.dirty,
};
const validityInput = append("lightning-input", Input, { label: "Validity", value: "Ada", disabled: false });
validityInput.setCustomValidity("Bad value");
window.__baseInputValidityContracts = {
  before: validityInput.checkValidity(),
  reported: validityInput.reportValidity(),
};
validityInput.setCustomValidity("");
window.__baseInputValidityContracts.after = validityInput.checkValidity();
validityInput.focus();
validityInput.blur();
append("lightning-textarea", Textarea, { label: "Notes", value: "Ready" });
append("lightning-combobox", Combobox, {
  label: "Stage",
  value: "open",
  options: [{ label: "Open", value: "open" }, { label: "Closed", value: "closed" }]
});
const card = append("lightning-card", Card, { title: "Account Card", iconName: "standard:account", variant: "narrow" });
const cardAction = document.createElement("button");
cardAction.slot = "actions";
cardAction.textContent = "Refresh";
card.appendChild(cardAction);
const cardBody = document.createElement("p");
cardBody.textContent = "Card body";
card.appendChild(cardBody);
const cardFooter = document.createElement("a");
cardFooter.slot = "footer";
cardFooter.href = "#";
cardFooter.textContent = "View All";
card.appendChild(cardFooter);
const layout = append("lightning-layout", Layout, { multipleRows: true, horizontalAlign: "spread", verticalAlign: "center", pullToBoundary: "small" });
const layoutItem = createElement("lightning-layout-item", { is: LayoutItem });
Object.assign(layoutItem, { size: 6, smallDeviceSize: 12, mediumDeviceSize: 6, largeDeviceSize: 4, padding: "around-small", flexibility: "auto" });
layoutItem.textContent = "Layout Body";
layout.appendChild(layoutItem);
const datatable = createElement("lightning-datatable", { is: Datatable });
Object.assign(datatable, {
  keyField: "id",
  columns: [
    { label: "Name", fieldName: "name" },
    { type: "action", typeAttributes: { rowActions: [{ label: "View", name: "view" }] } },
    { type: "action", typeAttributes: { rowActions: [{ label: "Edit", name: "edit" }] } }
  ],
  data: [{ id: "1", name: "Local Shell Account" }]
});
datatable.addEventListener("rowaction", (event) => {
  window.__baseRowAction = event.detail;
});
datatable.setAttribute("hide-checkbox-column", "true");
host.appendChild(datatable);
append("lightning-record-form", RecordForm, {
  objectApiName: "Account",
  recordId: "001000000000001AAA",
  fields: ["Name", "Phone"]
});
const editForm = append("lightning-record-edit-form", RecordEditForm, {
  objectApiName: "Account",
  recordId: "001000000000001AAA",
  fields: ["Name"],
  mode: "edit"
});
editForm.addEventListener("success", (event) => {
  window.__baseFormSuccess = event.detail;
});
editForm.addEventListener("error", (event) => {
  window.__baseFormError = event.detail;
});
const tabset = append("lightning-tabset", Tabset);
const tab = createElement("lightning-tab", { is: Tab });
tab.label = "Details";
tab.value = "details";
tab.addEventListener("active", (event) => {
  window.__baseActiveTab = event.detail;
});
tabset.appendChild(tab);
append("lightning-icon", Icon, { iconName: "utility:check", alternativeText: "checked" });
append("lightning-modal", Modal, { label: "Local Modal" });
const accordion = append("lightning-accordion", Accordion);
const accordionSection = createElement("lightning-accordion-section", { is: AccordionSection });
accordionSection.label = "Package Section";
accordion.appendChild(accordionSection);
append("lightning-avatar", Avatar, { initials: "LV", alternativeText: "Local Avatar" });
append("lightning-badge", Badge, { label: "Verified" });
const menu = append("lightning-button-menu", ButtonMenu, { label: "Package Actions" });
const menuItem = createElement("lightning-menu-item", { is: MenuItem });
menuItem.label = "Open";
menuItem.addEventListener("active", (event) => {
  window.__baseMenuActive = event.detail;
});
menu.appendChild(menuItem);
append("lightning-checkbox-group", CheckboxGroup, { label: "Checks", value: ["a"], options: [{ label: "A", value: "a" }] });
const upload = append("lightning-file-upload", FileUpload, { label: "Upload Evidence" });
upload.addEventListener("uploadfinished", (event) => {
  window.__baseUploadFinished = event.detail;
});
append("lightning-flow", Flow, { flowApiName: "Package_Flow" });
append("lightning-formatted-date-time", FormattedDateTime, { value: "2026-06-17T12:00:00Z" });
append("lightning-formatted-number", FormattedNumber, { value: 0.42, formatStyle: "percent", minimumFractionDigits: 0, maximumFractionDigits: 0 });
append("lightning-formatted-rich-text", FormattedRichText, { value: "<b>Rich Text</b>" });
append("lightning-helptext", Helptext, { content: "Package help" });
append("lightning-pill", Pill, { label: "Package Pill" });
append("lightning-quick-action-panel", QuickActionPanel, { header: "Quick Action" });
append("lightning-radio-group", RadioGroup, { label: "Choice", value: "yes", options: [{ label: "Yes", value: "yes" }] });
const recordPicker = append("lightning-record-picker", RecordPicker, { label: "Provider", value: "001000000000001AAA" });
recordPicker.addEventListener("change", (event) => {
  window.__baseRecordPicker = event.detail;
});
const nav = append("lightning-vertical-navigation", VerticalNavigation);
const navItem = createElement("lightning-vertical-navigation-item", { is: VerticalNavigationItem });
navItem.label = "Credentials";
nav.appendChild(navItem);
window.__baseDiagnostics = diagnostics;
window.__modalOpen = Modal.open({ label: "Local Modal", result: "done" });
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

test("base components render practical SLDS 2 DOM", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const server = await startBaseComponentServer();
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
    await page.goto(`${server.baseURL}/base.html`, { waitUntil: "networkidle" });
    const mounted = await page.locator("lightning-button").count();
    assert.equal(mounted, 1, `button did not mount; pageErrors=${JSON.stringify(pageErrors)} consoleErrors=${JSON.stringify(consoleErrors)}`);

    assert.match(await page.locator("lightning-button").innerText(), /Save/);
    assert.equal(await page.locator("lightning-button button").isEnabled(), true);
    const buttonClass = await page.locator("lightning-button button").getAttribute("class");
    assert.match(buttonClass, /slds-button_brand/);
    assert.equal(await page.locator("lightning-button button").getAttribute("name"), "saveButton");
    assert.equal(await page.locator("lightning-button button").getAttribute("value"), "save");
    await page.locator("lightning-button button", { hasText: "Save" }).click();
    assert.equal(await page.evaluate(() => window.__baseClickCount), 1);
    const iconButtonClass = await page.locator("lightning-button-icon button").getAttribute("class");
    assert.match(iconButtonClass, /slds-button_icon-border-filled/);
    assert.match(iconButtonClass, /slds-button_icon-small/);
    assert.equal(await page.locator("lightning-button-icon button").getAttribute("aria-label"), "Add Account");
    assert.equal(await page.locator("lightning-button-icon button").getAttribute("name"), "addAccount");
    const nameInput = page.locator("lightning-input").filter({ hasText: "Name" });
    assert.match(await nameInput.innerText(), /Name/);
    assert.equal(await nameInput.locator("input").isEnabled(), true);
    assert.equal(await nameInput.locator("input").inputValue(), "Ada");
    assert.deepEqual(await page.evaluate(() => window.__baseInputFieldContracts), {
      errors: { message: "Required" },
      wiredData: { record: { id: "001000000000001AAA" } },
      wiredPicklistValues: { Name: { values: [{ label: "Warm", value: "Warm" }] } },
      value: "Grace",
      dirty: true,
      dirtyAfterClean: false,
      resetValue: "Ada",
      dirtyAfterReset: false,
    });
    assert.deepEqual(await page.evaluate(() => window.__baseInputValidityContracts), {
      before: false,
      reported: false,
      after: true,
    });
    assert.equal(await page.locator("lightning-combobox select").inputValue(), "open");
    assert.match(await page.locator("lightning-card").innerText(), /Account Card/);
    assert.match(await page.locator("lightning-card").innerText(), /Refresh/);
    assert.match(await page.locator("lightning-card").innerText(), /View All/);
    assert.match(await page.locator("lightning-card article").getAttribute("class"), /slds-card_narrow/);
    assert.equal(await page.locator("lightning-card .slds-no-flex").count(), 1);
    assert.equal(await page.locator("lightning-card .slds-card__footer").count(), 1);
    const layoutClass = await page.locator("lightning-layout div.slds-grid").getAttribute("class");
    assert.match(layoutClass, /slds-grid_align-spread/);
    assert.match(layoutClass, /slds-grid_vertical-align-center/);
    assert.match(layoutClass, /slds-wrap/);
    const layoutItemClass = await page.locator("lightning-layout-item div.slds-col").getAttribute("class");
    assert.match(layoutItemClass, /slds-size_6-of-12/);
    assert.match(layoutItemClass, /slds-small-size_12-of-12/);
    assert.match(layoutItemClass, /slds-medium-size_6-of-12/);
    assert.match(layoutItemClass, /slds-large-size_4-of-12/);
    assert.match(layoutItemClass, /slds-p-around_small/);
    assert.match(await page.locator("lightning-datatable").innerText(), /Local Shell Account/);
    await page.locator("lightning-datatable button", { hasText: "View" }).click();
    assert.deepEqual(await page.evaluate(() => window.__baseRowAction), {
      action: { label: "View", name: "view" },
      row: { id: "1", name: "Local Shell Account" },
    });
    await page.locator("lightning-datatable button", { hasText: "Edit" }).click();
    assert.deepEqual(await page.evaluate(() => window.__baseRowAction), {
      action: { label: "Edit", name: "edit" },
      row: { id: "1", name: "Local Shell Account" },
    });
    assert.match(await page.locator("lightning-record-form").innerText(), /001000000000001AAA/);
    assert.match(await page.locator("lightning-record-form").innerText(), /Local Shell Account/);
    await page.locator("lightning-record-edit-form input").fill("Edited Local Account");
    await page.locator("lightning-record-edit-form button", { hasText: "Save" }).click();
    await page.waitForFunction(() => window.__baseFormSuccess);
    assert.deepEqual(await page.evaluate(() => window.__baseFormSuccess), {
      id: "001000000000001AAA",
      fields: { Name: "Edited Local Account" },
    });
    assert.equal(await page.evaluate(() => window.__baseFormError), undefined);
    assert.match(await page.locator("lightning-tabset").innerText(), /Details/);
    await page.locator("lightning-tab h3", { hasText: "Details" }).click();
    assert.deepEqual(await page.evaluate(() => window.__baseActiveTab), { value: "details", label: "Details" });
    assert.match(await page.locator("lightning-icon").innerText(), /check/);
    assert.match(await page.locator("lightning-modal").innerText(), /Local Modal/);
    assert.match(await page.locator("lightning-accordion").innerText(), /Package Section/);
    assert.match(await page.locator("lightning-avatar").innerText(), /LV/);
    assert.match(await page.locator("lightning-badge").innerText(), /Verified/);
    assert.match(await page.locator("lightning-button-menu").innerText(), /Package Actions/);
    await page.locator("lightning-menu-item button", { hasText: "Open" }).click();
    assert.deepEqual(await page.evaluate(() => window.__baseMenuActive), { value: "Open", label: "Open" });
    assert.match(await page.locator("lightning-checkbox-group").innerText(), /Checks/);
    assert.match(await page.locator("lightning-flow").innerText(), /Package_Flow/);
    assert.match(await page.locator("lightning-formatted-date-time").innerText(), /2026-06-17/);
    assert.match(await page.locator("lightning-formatted-number").innerText(), /^42%$/);
    assert.match(await page.locator("lightning-formatted-rich-text").innerText(), /Rich Text/);
    assert.match(await page.locator("lightning-helptext").innerText(), /Package help/);
    assert.match(await page.locator("lightning-pill").innerText(), /Package Pill/);
    assert.match(await page.locator("lightning-quick-action-panel").innerText(), /Quick Action/);
    assert.match(await page.locator("lightning-radio-group").innerText(), /Choice/);
    await page.locator("lightning-record-picker input").evaluate((input) => {
      input.value = "001000000000002AAA";
      input.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
    });
    assert.deepEqual(await page.evaluate(() => window.__baseRecordPicker), {
      recordId: "001000000000002AAA",
      value: "001000000000002AAA",
    });
    assert.match(await page.locator("lightning-vertical-navigation").innerText(), /Credentials/);
    assert.equal(await page.evaluate(() => window.__modalOpen), "done");
    assert.equal(await page.locator(`link[href="${defaultSLDSHref}"]`).count(), 1);
    assert.deepEqual(pageErrors, []);
    assert.deepEqual(consoleErrors, []);
  } finally {
    await browser.close();
    await server.close();
  }
});

test("package base component and missing SLDS asset record diagnostics", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const server = await startBaseComponentServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/base.html`, { waitUntil: "networkidle" });
    await page.evaluate(async () => {
      const { loadSLDS } = await import("@glade/slds");
      await loadSLDS({ theme: "missing" });
      const { createElement } = await import("lwc");
      const { default: FormattedLocation } = await import("/lightning/shims/lightning/formattedLocation.js");
      const { default: LightningToast } = await import("/lightning/shims/lightning/toast.js");
      const toastDetails = [];
      document.addEventListener("lightning__showtoast", (event) => toastDetails.push(event.detail));
      await LightningToast.show({ label: "Local Toast", message: "Ready" });
      const el = createElement("lightning-formatted-location", { is: FormattedLocation });
      el.latitude = "61.22";
      el.longitude = "-149.90";
      document.getElementById("host").appendChild(el);
      window.__baseToastDetails = toastDetails;
    });
    const diagnostics = await page.evaluate(() => window.__baseDiagnostics.map((d) => d.code));
    assert.equal(await page.locator("lightning-formatted-location").innerText(), "61.22, -149.90");
    assert.deepEqual(await page.evaluate(() => window.__baseToastDetails), [
      { label: "Local Toast", message: "Ready", source: undefined },
    ]);
    assert.equal(diagnostics.includes("GLADELWC060"), false, JSON.stringify(diagnostics));
    assert.ok(diagnostics.includes("GLADELWC061"), JSON.stringify(diagnostics));
    assert.ok(diagnostics.includes("GLADELWC062"), JSON.stringify(diagnostics));
  } finally {
    await browser.close();
    await server.close();
  }
});

test("datatable supports local row actions, selection, sorting, drafts, and value types", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const server = await startBaseComponentServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/base.html`, { waitUntil: "networkidle" });
    await page.evaluate(async () => {
      const { createElement } = await import("lwc");
      const { default: Datatable } = await import("/lightning/shims/lightning/datatable.js");
      const host = document.getElementById("host");
      host.innerHTML = "";
      const events = [];
      const datatable = createElement("lightning-datatable", { is: Datatable });
      datatable.keyField = "Id";
      datatable.columns = [
        { label: "Name", fieldName: "Name", type: "text", editable: true, sortable: true },
        { label: "Amount", fieldName: "Amount__c", type: "currency" },
        { label: "Ready", fieldName: "Ready__c", type: "boolean" },
        { label: "Website", fieldName: "Website__c", type: "url", typeAttributes: { label: { fieldName: "WebsiteLabel__c" } } },
        { label: "Email", fieldName: "Email__c", type: "email" },
        { label: "Phone", fieldName: "Phone__c", type: "phone" },
        { label: "Status", fieldName: "Status__c", type: "badge" },
        { type: "button", typeAttributes: { label: "Open", name: "open" } },
        { type: "action", typeAttributes: { rowActions: [{ label: "View", name: "view" }] } },
      ];
      datatable.data = [{
        Id: "001",
        Name: "Acme",
        Amount__c: 42.5,
        Ready__c: true,
        Website__c: "https://example.com",
        WebsiteLabel__c: "Example",
        Email__c: "hello@example.com",
        Phone__c: "415-555-0100",
        Status__c: "Ready",
      }];
      datatable.selectedRows = ["001"];
      datatable.addEventListener("rowaction", (event) => events.push(["rowaction", event.detail]));
      datatable.addEventListener("sort", (event) => events.push(["sort", event.detail]));
      datatable.addEventListener("rowselection", (event) => events.push(["rowselection", event.detail]));
      datatable.addEventListener("cellchange", (event) => events.push(["cellchange", event.detail]));
      datatable.addEventListener("save", (event) => events.push(["save", event.detail]));
      datatable.addEventListener("cancel", (event) => events.push(["cancel", event.detail]));
      datatable.addEventListener("loadmore", (event) => events.push(["loadmore", event.detail]));
      host.appendChild(datatable);
      window.__datatableEvents = events;
      window.__datatableSelected = datatable.getSelectedRows();
    });

    const table = page.locator("lightning-datatable");
    assert.match(await table.innerText(), /Acme/);
    assert.match(await table.innerText(), /\$42\.50/);
    assert.match(await table.innerText(), /Ready/);
    assert.equal(await table.locator('a[href="https://example.com"]').innerText(), "Example");
    assert.equal(await table.locator('a[href="mailto:hello@example.com"]').innerText(), "hello@example.com");
    assert.equal(await table.locator('a[href="tel:415-555-0100"]').innerText(), "415-555-0100");
    assert.deepEqual(await page.evaluate(() => window.__datatableSelected), [{
      Id: "001",
      Name: "Acme",
      Amount__c: 42.5,
      Ready__c: true,
      Website__c: "https://example.com",
      WebsiteLabel__c: "Example",
      Email__c: "hello@example.com",
      Phone__c: "415-555-0100",
      Status__c: "Ready",
    }]);

    await table.locator("button", { hasText: "Name" }).click();
    await table.locator("input[type=\"checkbox\"]").first().setChecked(false);
    await table.locator("input[data-field-name=\"Name\"]").fill("Acme West");
    await table.locator("input[data-field-name=\"Name\"]").dispatchEvent("change");
    await table.locator("button", { hasText: "Save" }).click();
    await table.locator("button", { hasText: "Cancel" }).click();
    await table.locator("button", { hasText: "View" }).click();
    await table.locator("button", { hasText: "Open" }).click();
    await table.locator("button", { hasText: "Load More" }).click();

    const events = await page.evaluate(() => window.__datatableEvents);
    assert.deepEqual(events.find(([name]) => name === "sort"), ["sort", {
      fieldName: "Name",
      sortedBy: "Name",
      sortDirection: "asc",
    }]);
    assert.deepEqual(events.find(([name]) => name === "rowselection"), ["rowselection", {
      selectedRows: [],
      selectedRowKeys: [],
    }]);
    assert.deepEqual(events.find(([name]) => name === "cellchange"), ["cellchange", {
      draftValues: [{ Id: "001", Name: "Acme West" }],
    }]);
    assert.deepEqual(events.find(([name]) => name === "save"), ["save", {
      draftValues: [{ Id: "001", Name: "Acme West" }],
    }]);
    assert.deepEqual(events.find(([name]) => name === "cancel"), ["cancel", {
      draftValues: [],
    }]);
    assert.deepEqual(events.filter(([name]) => name === "rowaction").map(([, detail]) => detail), [
      { action: { label: "View", name: "view" }, row: { Id: "001", Name: "Acme", Amount__c: 42.5, Ready__c: true, Website__c: "https://example.com", WebsiteLabel__c: "Example", Email__c: "hello@example.com", Phone__c: "415-555-0100", Status__c: "Ready" } },
      { action: { label: "Open", name: "open" }, row: { Id: "001", Name: "Acme", Amount__c: 42.5, Ready__c: true, Website__c: "https://example.com", WebsiteLabel__c: "Example", Email__c: "hello@example.com", Phone__c: "415-555-0100", Status__c: "Ready" } },
    ]);
    assert.deepEqual(events.find(([name]) => name === "loadmore"), ["loadmore", {}]);
  } finally {
    await browser.close();
    await server.close();
  }
});

test("record forms and fields support LDS endpoints, validity, reset, and picklists", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }
  const server = await startBaseComponentServer();
  const browser = await chromium.launch({ headless: true });
  try {
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/base.html`, { waitUntil: "networkidle" });
    await page.evaluate(async () => {
      const { createElement } = await import("lwc");
      const { default: InputField } = await import("/lightning/shims/lightning/inputField.js");
      const { default: OutputField } = await import("/lightning/shims/lightning/outputField.js");
      const { default: RecordForm } = await import("/lightning/shims/lightning/recordForm.js");
      const { default: RecordEditForm } = await import("/lightning/shims/lightning/recordEditForm.js");
      const { default: RecordViewForm } = await import("/lightning/shims/lightning/recordViewForm.js");
      const { default: Messages } = await import("/lightning/shims/lightning/messages.js");
      const host = document.getElementById("host");
      host.innerHTML = "";
      const events = [];

      const input = createElement("lightning-input-field", { is: InputField });
      Object.assign(input, {
        fieldName: "Type",
        label: "Type",
        value: "Customer",
        required: true,
        options: [{ label: "Customer", value: "Customer" }, { label: "Partner", value: "Partner" }],
      });
      host.appendChild(input);
      const emptyRequired = createElement("lightning-input-field", { is: InputField });
      Object.assign(emptyRequired, { fieldName: "Name", label: "Account Name", value: "", required: true });
      host.appendChild(emptyRequired);

      const output = createElement("lightning-output-field", { is: OutputField });
      Object.assign(output, { fieldName: "Name", value: "Local Shell Account" });
      host.appendChild(output);

      const recordForm = createElement("lightning-record-form", { is: RecordForm });
      Object.assign(recordForm, { objectApiName: "Account", recordId: "001000000000001AAA", fields: ["Name", "Phone"] });
      recordForm.addEventListener("load", (event) => events.push(["recordformload", event.detail.record.id]));
      host.appendChild(recordForm);

      const viewForm = createElement("lightning-record-view-form", { is: RecordViewForm });
      Object.assign(viewForm, { objectApiName: "Account", recordId: "001000000000001AAA" });
      viewForm.appendChild(output.cloneNode(true));
      host.appendChild(viewForm);

      const editForm = createElement("lightning-record-edit-form", { is: RecordEditForm });
      Object.assign(editForm, { objectApiName: "Account", recordId: "001000000000001AAA" });
      const nameField = createElement("lightning-input-field", { is: InputField });
      Object.assign(nameField, { fieldName: "Name", value: "Local Shell Account", required: true });
      editForm.appendChild(nameField);
      const formMessages = createElement("lightning-messages", { is: Messages });
      editForm.appendChild(formMessages);
      editForm.addEventListener("load", (event) => events.push(["load", event.detail.record.id]));
      editForm.addEventListener("submit", (event) => events.push(["submit", event.detail.fields]));
      editForm.addEventListener("success", (event) => events.push(["success", event.detail.id]));
      editForm.addEventListener("error", (event) => events.push(["error", event.detail.message]));
      editForm.addEventListener("cancel", (event) => events.push(["cancel", event.detail.fields]));
      host.appendChild(editForm);

      input.setCustomValidity("Pick a better type");
      const customInvalid = input.checkValidity();
      const customReported = input.reportValidity();
      input.setCustomValidity("");
      input.value = "Partner";
      input.reset();
      input.focus();
      input.blur();

      window.__recordFormEvents = events;
      window.__fieldContracts = {
        customInvalid,
        customReported,
        validAfterClear: input.checkValidity(),
        emptyRequired: emptyRequired.reportValidity(),
        resetValue: input.value,
        picklistOptions: input.options,
      };
    });

    await page.waitForFunction(() => window.__recordFormEvents.some(([name]) => name === "load"));
    assert.match(await page.locator("lightning-output-field").first().innerText(), /Local Shell Account/);
    assert.equal(await page.locator("lightning-input-field").first().locator("select").inputValue(), "Customer");
    assert.deepEqual(await page.evaluate(() => window.__fieldContracts), {
      customInvalid: false,
      customReported: false,
      validAfterClear: true,
      emptyRequired: false,
      resetValue: "Customer",
      picklistOptions: [{ label: "Customer", value: "Customer" }, { label: "Partner", value: "Partner" }],
    });

    await page.locator("lightning-record-edit-form lightning-input-field input").fill("Edited Local Account");
    await page.locator("lightning-record-edit-form button", { hasText: "Save" }).click();
    await page.waitForFunction(() => window.__recordFormEvents.some(([name]) => name === "success"));
    await page.locator("lightning-record-edit-form button", { hasText: "Cancel" }).click();

    const events = await page.evaluate(() => window.__recordFormEvents);
    assert.ok(events.some(([name, value]) => name === "recordformload" && value === "001000000000001AAA"), JSON.stringify(events));
    assert.ok(events.some(([name, value]) => name === "load" && value === "001000000000001AAA"), JSON.stringify(events));
    assert.deepEqual(events.find(([name]) => name === "submit"), ["submit", { Name: "Edited Local Account" }]);
    assert.deepEqual(events.find(([name]) => name === "success"), ["success", "001000000000001AAA"]);
    assert.deepEqual(events.find(([name]) => name === "cancel"), ["cancel", { Name: "Edited Local Account" }]);
    assert.equal(events.some(([name]) => name === "error"), false, JSON.stringify(events));
  } finally {
    await browser.close();
    await server.close();
  }
});
