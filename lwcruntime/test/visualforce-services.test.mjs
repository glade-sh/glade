import assert from "node:assert/strict";
import test from "node:test";
import { chromium } from "playwright";
import { requireLWCToolchain, startVisualforceDevServer } from "./helpers.mjs";

const fixture = "testdata/local-tests/lightning-out-vf";

test("Visualforce Lightning Out provides local LWC service shims", async (t) => {
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

    const host = page.locator("c-service-host");
    assert.equal(await host.locator(".page-type").innerText({ timeout: 10000 }), "standard__webPage");
    assert.equal(await host.locator(".toast-title").innerText(), "VF Toast");
    assert.equal(await host.locator(".message-record").innerText(), "001XX0000000001");
    assert.equal(await host.locator(".resource-status").innerText(), "loaded");
    assert.equal(await host.locator(".nav-error").innerText(), "GLADELWC042");

    const callbacks = await page.evaluate(() => window.__callbacks);
    assert.equal(callbacks["c:serviceHost"].status, "SUCCESS");
    assert.equal(callbacks["c:serviceHost"].errorMessage, undefined);
  } finally {
    if (browser) {
      await browser.close();
    }
    await server.close();
  }
});
