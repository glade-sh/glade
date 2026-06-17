import assert from "node:assert/strict";
import test from "node:test";
import { chromium } from "playwright";
import { requireLWCToolchain, startVisualforceDevServer } from "./helpers.mjs";

const fixture = "testdata/local-tests/lightning-out-vf";

test("MultiWidgetHost boots Lightning Out components in the rendered Visualforce page", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }

  const server = await startVisualforceDevServer(t, {
    projectRel: fixture,
    pagePath: "/apex/MultiWidgetHost",
  });
  if (!server) {
    return;
  }
  assert.ok(server.pages.includes("/apex/MultiWidgetHost"));

  let browser;
  const pageErrors = [];
  const consoleErrors = [];
  try {
    browser = await chromium.launch({ headless: true });
    const page = await browser.newPage();
    page.on("pageerror", (err) => {
      pageErrors.push(err.message);
    });
    page.on("console", (msg) => {
      if (msg.type() === "error") {
        consoleErrors.push(msg.text());
      }
    });

    await page.goto(`${server.baseURL}/apex/MultiWidgetHost`, { waitUntil: "networkidle" });

    assert.equal(await page.locator('c-context-probe [data-probe="vf-context"] h2').innerText(), "VF Explicit Context");
    assert.equal(await page.locator('c-context-probe [data-probe="vf-context"] p').first().innerText(), "001XX0000000001");
    assert.equal(await page.locator('c-wire-probe [data-probe="vf-wire"] p').nth(1).innerText({ timeout: 10000 }), "Local Widget");
    assert.equal(await page.locator('c-wire-probe [data-probe="vf-wire"] p').nth(2).innerText(), "Local Widget");
    assert.equal(
      await page.locator("c-apex-wire-host .items").innerText({ timeout: 10000 }),
      "Local Widget",
    );
    assert.equal(await page.locator("c-record-wire-host .name").innerText(), "Acme");
    assert.equal(await page.locator("c-object-info-host .label").innerText({ timeout: 10000 }), "Account");
    assert.equal(
      await page.locator("c-record-mutation-host .status").innerText({ timeout: 10000 }),
      "mutation complete",
    );
    assert.equal(await page.locator("c-label-resource-host .label").innerText(), "Hello from Glade");
    assert.equal(await page.locator("c-label-resource-host .resource").innerText(), "/resource/WidgetAssets");
    assert.equal(await page.locator("c-service-host .page-type").innerText({ timeout: 10000 }), "standard__webPage");
    assert.equal(await page.locator("c-service-host .toast-title").innerText(), "VF Toast");
    assert.equal(await page.locator("c-service-host .message-record").innerText(), "001XX0000000001");
    assert.equal(await page.locator("c-service-host .resource-status").innerText(), "loaded");
    assert.equal(await page.locator("c-service-host .nav-error").innerText(), "GLADELWC042");
    assert.match(await page.locator('c-base-component-host [data-probe="vf-base"] lightning-card').innerText(), /VF Base Card/);
    assert.match(await page.locator('c-base-component-host [data-probe="vf-base"] lightning-button').innerText(), /Save VF/);
    assert.equal(await page.locator('c-base-component-host [data-probe="vf-base"] lightning-input input').inputValue(), "Ada");
    assert.match(await page.locator('c-base-component-host [data-probe="vf-base"] lightning-datatable').innerText(), /VF Local Account/);
    assert.match(await page.locator('c-base-component-host [data-probe="vf-base"] lightning-record-form').innerText({ timeout: 10000 }), /Acme/);
    assert.match(await page.locator('c-base-component-host [data-probe="vf-base"] lightning-tabset').innerText(), /Details/);
    await page.locator('c-base-component-host [data-probe="vf-base"] lightning-tab h3', { hasText: "Details" }).click();
    assert.equal(await page.locator('c-base-component-host [data-probe="vf-base"] .tab-status').innerText(), "details");
    assert.equal(await page.locator('link[href="/lightning/runtime/slds/glade-slds.css"]').count(), 1);

    const callbacks = await page.evaluate(() => window.__callbacks);
    for (const qualified of [
      "c:contextProbe",
      "c:wireProbe",
      "c:apexWireHost",
      "c:recordWireHost",
      "c:objectInfoHost",
      "c:recordMutationHost",
      "c:labelResourceHost",
      "c:serviceHost",
      "c:eventChild",
      "c:baseComponentHost",
    ]) {
      assert.equal(callbacks[qualified].status, "SUCCESS", qualified);
      assert.equal(callbacks[qualified].errorMessage, undefined, qualified);
    }

    await page.locator("c-event-child button").click();
    assert.equal(await page.evaluate(() => window.__selected), "1");
    assert.deepEqual(pageErrors, []);
    assert.deepEqual(consoleErrors, []);
  } finally {
    if (browser) {
      await browser.close();
    }
    await server.close();
  }
});
