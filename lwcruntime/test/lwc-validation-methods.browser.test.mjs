import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import { startValidationMethodsServer } from "./validation-methods-harness.mjs";

const playwrightPath = process.env.GLADE_LWC_PLAYWRIGHT_MODULE;
const playwrightModule = playwrightPath
  ? await import(pathToFileURL(path.extname(playwrightPath) ? playwrightPath : path.join(playwrightPath, "index.mjs")).href)
  : await import("playwright");
const { chromium } = playwrightModule;

async function clickAndReadState(page, buttonName, expectedStep) {
  await page.getByRole("button", { name: buttonName, exact: true }).click();
  const state = page.getByTestId("validation-state")
    .filter({ hasText: '"step":"' + expectedStep + '"' });
  await state.waitFor({ state: "visible" });
  return JSON.parse(await state.innerText());
}

test("compiled LWC consumer exercises lightning-textarea required and custom validity methods", async () => {
  const server = await startValidationMethodsServer();
  let browser;
  try {
    assert.equal(server.framework.declaredComponentApiVersion, 67);
    assert.deepEqual(
      [server.framework.compilerVersion, server.framework.engineDomVersion, server.framework.syntheticShadowVersion],
      ["9.4.3", "9.4.3", "9.4.3"],
      "framework package versions are local toolchain identity, not Salesforce API versions",
    );
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
    await page.waitForFunction(() => window.__validationMethodsMounted === true);

    assert.deepEqual(
      await clickAndReadState(page, "Check required", "required-empty"),
      { step: "required-empty", required: true, value: "", check: false, report: false },
    );
    assert.deepEqual(
      await clickAndReadState(page, "Set nonempty value", "property-set"),
      { step: "property-set", required: true, value: "ready" },
    );
    assert.deepEqual(
      await clickAndReadState(page, "Check valid value", "value-valid"),
      { step: "value-valid", required: true, value: "ready", check: true, report: true },
    );
    assert.deepEqual(
      await clickAndReadState(page, "Set custom error", "custom-error"),
      { step: "custom-error", value: "ready", check: false, report: false },
    );
    assert.deepEqual(
      await clickAndReadState(page, "Clear custom error", "custom-error-cleared"),
      { step: "custom-error-cleared", value: "ready", check: true, report: true },
    );
    assert.deepEqual(pageErrors, []);
  } finally {
    await browser?.close();
    await server.close();
  }
});
