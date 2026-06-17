import assert from "node:assert/strict";
import test from "node:test";
import { chromium } from "playwright";
import { requireLWCToolchain, startVisualforceDevServer } from "./helpers.mjs";

const fixture = "testdata/local-tests/lightning-out-vf";

test("Visualforce Lightning Out renders supported base components with SLDS", async (t) => {
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

  let browser;
  try {
    browser = await chromium.launch({ headless: true });
    const page = await browser.newPage();
    await page.goto(`${server.baseURL}/apex/MultiWidgetHost`, { waitUntil: "networkidle" });

    const host = page.locator('c-base-component-host [data-probe="vf-base"]');
    assert.match(await host.locator("lightning-card").innerText({ timeout: 10000 }), /VF Base Card/);
    assert.match(await host.locator("lightning-button").innerText(), /Save VF/);
    assert.equal(await host.locator("lightning-input input").inputValue(), "Ada");
    assert.match(await host.locator("lightning-datatable").innerText(), /VF Local Account/);
    assert.match(await host.locator("lightning-record-form").innerText({ timeout: 10000 }), /Acme/);
    assert.match(await host.locator("lightning-tabset").innerText(), /Details/);
    await host.locator("lightning-tab h3", { hasText: "Details" }).click();
    assert.equal(await host.locator(".tab-status").innerText(), "details");
    assert.equal(await page.locator("link[data-glade-slds]").count(), 1);
  } finally {
    if (browser) {
      await browser.close();
    }
    await server.close();
  }
});
