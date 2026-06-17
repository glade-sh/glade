import assert from "node:assert/strict";
import test from "node:test";
import { chromium } from "playwright";
import { requireLWCToolchain, startLWCDevServer } from "./helpers.mjs";

const fixture = "testdata/local-tests/lwc-shell";

test("LWC dev shell renders a record FlexiPage with local data and shims", async (t) => {
  if (!requireLWCToolchain(t)) {
    return;
  }

  const route = "/lwc/preview/record/Account/001000000000001AAA?page=Account_Record_Page";
  const server = await startLWCDevServer(t, {
    projectRel: fixture,
    pagePath: route,
  });
  if (!server) {
    return;
  }
  assert.ok(server.routes.includes("/lwc/preview/record/Account/<recordId>?page=Account_Record_Page"));

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

    await page.goto(`${server.baseURL}${route}`, { waitUntil: "networkidle" });

    assert.match(await page.locator("c-context-probe").innerText({ timeout: 10000 }), /Account Context/);
    assert.match(await page.locator("c-context-probe").innerText(), /001000000000001AAA/);
    assert.match(await page.locator("c-record-probe").innerText(), /Local Shell Account/);
    assert.deepEqual(pageErrors, []);
    assert.deepEqual(consoleErrors, []);
  } finally {
    if (browser) {
      await browser.close();
    }
    await server.close();
  }
});
