import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import { startMenuNavigationServer } from "./menu-navigation-harness.mjs";

const playwrightPath = process.env.GLADE_LWC_PLAYWRIGHT_MODULE;
const playwrightModule = playwrightPath
  ? await import(pathToFileURL(path.extname(playwrightPath) ? playwrightPath : path.join(playwrightPath, "index.mjs")).href)
  : await import("playwright");
const { chromium } = playwrightModule;

async function readDiagnostics(page) {
  return JSON.parse(await page.locator("#event-diagnostics").innerText());
}

function matchingEvents(events, component, type, menuLabel) {
  return events.filter((event) => event.component === component
    && event.type === type
    && (menuLabel === undefined || event.menuLabel === menuLabel));
}

test("button menu and vertical navigation events follow real framework interactions", async () => {
  const server = await startMenuNavigationServer();
  let browser;
  try {
    assert.equal(server.framework.engineDomVersion, "9.4.3");
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

    await page.getByRole("button", { name: "Toggle actions" }).click();
    await page.getByRole("button", { name: "Toggle actions" }).click();
    await page.getByRole("button", { name: "Select action" }).click();
    await page.getByRole("menuitem", { name: "Approve" }).click();
    await page.getByRole("button", { name: "Cancel action" }).click();
    await page.getByRole("menuitem", { name: "Do not apply" }).click();
    await page.getByRole("link", { name: "Allowed" }).click();
    await page.getByRole("link", { name: "Blocked" }).click();

    const diagnosticsLocator = page.locator("#event-diagnostics");
    await diagnosticsLocator.waitFor({ state: "visible" });
    await diagnosticsLocator.filter({ hasText: '"interactionNumber": 8' })
      .waitFor({ state: "visible" });
    // Read the same rendered diagnostics panel that a person can inspect in the browser.
    assert.match(await diagnosticsLocator.innerText(), /"interactionNumber": 8/);
    const diagnostics = await readDiagnostics(page);
    assert.equal(diagnostics.framework.engineDomVersion, "9.4.3");

    const menuEvents = diagnostics.events.filter(({ component }) => component === "lightning-button-menu");
    for (const event of menuEvents) {
      if (event.type === "open" || event.type === "close") {
        assert.equal(event.detail, null);
        assert.deepEqual(event.flags, { bubbles: false, composed: false, cancelable: false });
        assert.equal(event.defaultPrevented, false);
      } else {
        assert.equal(event.type, "select");
        assert.equal(typeof event.detail?.value, "string");
        assert.deepEqual(Object.keys(event.detail), ["value"]);
        assert.deepEqual(event.flags, { bubbles: false, composed: false, cancelable: true });
      }
    }

    assert.equal(matchingEvents(menuEvents, "lightning-button-menu", "open", "Toggle actions").length, 1);
    assert.equal(matchingEvents(menuEvents, "lightning-button-menu", "close", "Toggle actions").length, 1);
    assert.equal(matchingEvents(menuEvents, "lightning-button-menu", "open", "Select action").length, 1);
    const selectedMenuEvent = matchingEvents(menuEvents, "lightning-button-menu", "select", "Select action")[0];
    assert.deepEqual(selectedMenuEvent.detail, { value: "approve" });
    assert.equal(selectedMenuEvent.defaultPrevented, false);
    assert.equal(matchingEvents(menuEvents, "lightning-button-menu", "open", "Cancel action").length, 1);
    const cancelledMenuEvent = matchingEvents(menuEvents, "lightning-button-menu", "select", "Cancel action")[0];
    assert.deepEqual(cancelledMenuEvent.detail, { value: "" });
    assert.equal(cancelledMenuEvent.defaultPrevented, true);

    const navigationEvents = diagnostics.events
      .filter(({ component }) => component === "lightning-vertical-navigation");
    for (const event of navigationEvents) {
      assert.ok(event.type === "beforeselect" || event.type === "select");
      assert.equal(typeof event.detail?.name, "string");
      assert.deepEqual(Object.keys(event.detail), ["name"]);
      assert.deepEqual(event.flags, event.type === "beforeselect"
        ? { bubbles: false, composed: false, cancelable: true }
        : { bubbles: false, composed: false, cancelable: false });
      if (event.type === "select") {
        assert.equal(event.defaultPrevented, false);
      }
    }
    const allowedBefore = matchingEvents(navigationEvents, "lightning-vertical-navigation", "beforeselect")
      .find(({ detail }) => detail.name === "");
    const allowedSelect = matchingEvents(navigationEvents, "lightning-vertical-navigation", "select")
      .find(({ detail }) => detail.name === "");
    const cancelledBefore = matchingEvents(navigationEvents, "lightning-vertical-navigation", "beforeselect")
      .find(({ detail }) => detail.name === "blocked");
    assert.equal(allowedBefore.defaultPrevented, false);
    assert.equal(allowedSelect.defaultPrevented, false);
    assert.equal(cancelledBefore.defaultPrevented, true);
    // control_legacy_menu_navigation: canceled beforeselect emits no select.
    assert.equal(matchingEvents(navigationEvents, "lightning-vertical-navigation", "select")
      .filter(({ detail }) => detail.name === "blocked").length, 0);

    // control_legacy_menu_navigation: all custom events stay on their hosts.
    assert.deepEqual(diagnostics.outerEvents, []);
    assert.equal(diagnostics.postInteractionObservations.length, 8);
    for (const observation of diagnostics.postInteractionObservations) {
      assert.ok(Array.isArray(observation.emittedEventsSincePriorClick));
      assert.match(observation.interpretation, /Observed sequence only/);
      assert.ok(observation.renderedDomState);
    }
    assert.equal(diagnostics.renderedDomState.menus.length, 3);
    assert.equal(diagnostics.renderedDomState.navigationItems.length, 2);
    assert.ok(diagnostics.renderedDomState.menus.flatMap(({ items }) => items)
      .every((item) => Object.hasOwn(item, "selectedProperty") && Object.hasOwn(item, "rendered")));
    assert.ok(diagnostics.renderedDomState.navigationItems
      .every((item) => Object.hasOwn(item, "selectedProperty") && Object.hasOwn(item, "rendered")));
    assert.deepEqual(pageErrors, []);
  } finally {
    await browser?.close();
    await server.close();
  }
});
