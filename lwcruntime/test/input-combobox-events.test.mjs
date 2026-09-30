import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import { startInputComboboxServer } from "./input-combobox-harness.mjs";

const playwrightPath = process.env.GLADE_LWC_PLAYWRIGHT_MODULE;
const playwrightModule = playwrightPath
  ? await import(pathToFileURL(path.extname(playwrightPath) ? playwrightPath : path.join(playwrightPath, "index.mjs")).href)
  : await import("playwright");
const { chromium } = playwrightModule;

async function readDiagnostics(page) {
  return JSON.parse(await page.locator("#event-diagnostics").innerText());
}

const expectedFlags = {
  "lightning-input|change": { bubbles: true, composed: true, cancelable: false },
  "lightning-input|commit": { bubbles: false, composed: false, cancelable: false },
  "lightning-combobox|change": { bubbles: true, composed: true, cancelable: false },
  "lightning-combobox|open": { bubbles: false, composed: false, cancelable: false },
};

test("input and combobox public events are observed through the retained LWC engine", async () => {
  const server = await startInputComboboxServer();
  let browser;
  try {
    assert.equal(server.framework.engineDomVersion, "9.4.3");
    assert.equal(server.framework.syntheticShadowVersion, "9.4.3");
    assert.equal(server.framework.engineAxis, "local framework/toolchain version; not Salesforce API version");
    browser = await chromium.launch({
      headless: true,
      ...(process.env.GLADE_LWC_BROWSER_EXECUTABLE
        ? { executablePath: process.env.GLADE_LWC_BROWSER_EXECUTABLE }
        : {}),
    });
    const page = await browser.newPage();
    const pageErrors = [];
    page.on("pageerror", (error) => pageErrors.push(error.message));
    await page.goto(server.url, { waitUntil: "networkidle" });

    const input = page.getByLabel("Text value");
    await input.fill("north value");
    await input.blur();
    await input.focus();
    await input.blur();
    await input.fill("");
    await input.blur();

    const combobox = page.getByLabel("Choice");
    await combobox.click();
    await combobox.selectOption("south");

    const diagnosticsLocator = page.locator("#event-diagnostics");
    await diagnosticsLocator.waitFor({ state: "visible" });
    await diagnosticsLocator.filter({ hasText: '"component": "lightning-combobox"' })
      .waitFor({ state: "visible" });
    const diagnostics = await readDiagnostics(page);
    assert.equal(diagnostics.renderedState.input.renderedValue, "");
    assert.equal(diagnostics.renderedState.combobox.renderedValue, "south");

    const events = diagnostics.events;
    for (const event of events) {
      const identity = `${event.component}|${event.type}`;
      assert.ok(Object.hasOwn(expectedFlags, identity), `unexpected public event ${identity}`);
      assert.deepEqual(event.flags, expectedFlags[identity]);
      assert.equal(event.defaultPrevented, false, `${identity} is documented noncancelable`);
      if (event.type === "commit" || event.type === "open") {
        assert.equal(event.detail, null);
      }
    }

    const inputChanges = events.filter(({ component, type }) => component === "lightning-input" && type === "change");
    assert.ok(inputChanges.some(({ detail }) => detail?.value === "north value"));
    assert.ok(inputChanges.some(({ detail }) => detail?.value === ""));
    const inputCommits = events.filter(({ component, type }) => component === "lightning-input" && type === "commit");
    assert.equal(inputCommits.length, 2, "each distinct text value commits once; unchanged blur does not repeat commit");
    const comboboxChanges = events.filter(({ component, type }) => component === "lightning-combobox" && type === "change");
    assert.ok(comboboxChanges.some(({ detail }) => detail?.value === "south"));
    assert.ok(events.some(({ component, type }) => component === "lightning-combobox" && type === "open"));

    const outerChanges = diagnostics.outerEvents.filter(({ type }) => type === "change");
    assert.ok(outerChanges.some(({ target }) => target === "lightning-input"));
    assert.ok(outerChanges.some(({ target }) => target === "lightning-combobox"));
    assert.ok(diagnostics.outerEvents.every(({ type }) => type === "change"));
    assert.deepEqual(pageErrors, []);
  } finally {
    await browser?.close();
    await server.close();
  }
});
