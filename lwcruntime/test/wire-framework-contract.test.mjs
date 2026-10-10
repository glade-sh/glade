import assert from "node:assert/strict";
import fs from "node:fs";
import path from "node:path";
import test from "node:test";
import { fileURLToPath, pathToFileURL } from "node:url";
import {
  startWireFrameworkHarness,
  wireFrameworkDeniedRecordId,
  wireFrameworkFirstRecordId,
} from "./wire-framework-harness.mjs";

const playwrightVersion = "1.62.1";

test("compiled wire function renders immutable data and reactive error outcomes", async () => {
  const browserExecutable = process.env.GLADE_LWC_BROWSER_EXECUTABLE;
  assert.ok(browserExecutable, "resource_guard must set GLADE_LWC_BROWSER_EXECUTABLE");
  assert.ok(fs.existsSync(browserExecutable), `guarded browser executable is missing: ${browserExecutable}`);
  const playwrightDir = process.env.GLADE_LWC_PLAYWRIGHT_MODULE;
  const playwrightEntry = playwrightDir
    ? path.join(playwrightDir, "index.mjs")
    : fileURLToPath(import.meta.resolve("playwright"));
  assert.ok(fs.existsSync(playwrightEntry), `Playwright entry is missing: ${playwrightEntry}`);
  const playwrightPackage = JSON.parse(fs.readFileSync(path.join(path.dirname(playwrightEntry), "package.json"), "utf8"));
  assert.equal(playwrightPackage.version, playwrightVersion, "unexpected Playwright version");
  const { chromium } = await import(playwrightDir ? pathToFileURL(playwrightEntry).href : "playwright");

  const harness = await startWireFrameworkHarness();
  let browser;
  try {
    browser = await chromium.launch({ executablePath: browserExecutable, headless: true });
    const page = await browser.newPage();
    await page.goto(harness.pageURL, { waitUntil: "networkidle" });
    await page.getByText("data:Wire Canary", { exact: true }).waitFor({ timeout: 10000 });

    const firstHistory = await page.locator('[data-testid="delivery-history"] li').allTextContents();
    assert.ok(firstHistory.some((text) => text.endsWith(":empty")), "initial function delivery must be visible");
    assert.ok(firstHistory.some((text) => text.endsWith(":data")), "configured data delivery must be visible");
    assert.equal((await page.locator('[data-testid="wire-name"]').textContent()).trim(), "Wire Canary");

    await page.getByRole("button", { name: "Attempt nested mutation" }).click();
    await page.getByText("mutation-after: Wire Canary", { exact: true }).waitFor({ timeout: 10000 });
    assert.equal((await page.locator('[data-testid="mutation-before"]').textContent()).trim(), "mutation-before: Wire Canary");
    assert.equal((await page.locator('[data-testid="mutation-after"]').textContent()).trim(), "mutation-after: Wire Canary");
    assert.equal((await page.locator('[data-testid="wire-name"]').textContent()).trim(), "Wire Canary");

    await page.getByRole("button", { name: "Request denied record" }).click();
    await page.getByText("error:record access denied", { exact: true }).waitFor({ timeout: 10000 });
    assert.equal((await page.locator('[data-testid="active-record"]').textContent()).trim(), `record-id: ${wireFrameworkDeniedRecordId}`);
    assert.equal((await page.locator('[data-testid="wire-name"]').textContent()).trim(), "");
    const finalHistory = await page.locator('[data-testid="delivery-history"] li').allTextContents();
    assert.ok(finalHistory.some((text) => text.endsWith(":error")), "reactive error delivery must be visible");

    const requestLog = await page.evaluate(async (url) => {
      const response = await fetch(url);
      if (!response.ok) throw new Error(`request log HTTP ${response.status}`);
      return response.json();
    }, harness.requestLogURL);
    assert.deepEqual(requestLog.requests, [
      { recordId: wireFrameworkFirstRecordId },
      { recordId: wireFrameworkDeniedRecordId },
    ]);
  } finally {
    await browser?.close();
    await harness.close();
  }
});
