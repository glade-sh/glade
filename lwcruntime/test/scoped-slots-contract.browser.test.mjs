import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import { compileShadowModeDiagnostic, startScopedSlotsServer } from "./scoped-slots-harness.mjs";

const playwrightPath = process.env.GLADE_LWC_PLAYWRIGHT_MODULE;
const playwrightModule = playwrightPath
  ? await import(pathToFileURL(path.extname(playwrightPath) ? playwrightPath : path.join(playwrightPath, "index.mjs")).href)
  : await import("playwright");
const { chromium } = playwrightModule;

test("API 67 compiled light-DOM child delivers scoped slot data to its parent", async () => {
  const server = await startScopedSlotsServer();
  let browser;
  try {
    assert.equal(server.framework.declaredComponentApiVersion, 67);
    assert.deepEqual(
      [server.framework.compilerVersion, server.framework.engineDomVersion, server.framework.syntheticShadowVersion],
      ["9.4.3", "9.4.3", "9.4.3"],
      "local framework package versions are not Salesforce API observations",
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
    await page.waitForFunction(() => window.__scopedSlotsMounted === true);

    const slotValue = page.getByTestId("scoped-slot-value");
    await slotValue.waitFor({ state: "visible" });
    assert.equal(await slotValue.innerText(), "child-provided-slot-data");
    assert.deepEqual(pageErrors, []);
  } finally {
    try {
      await browser?.close();
    } finally {
      await server.close();
    }
  }
});

test("compiler diagnoses scoped slot binding in a shadow-DOM child", () => {
  const result = compileShadowModeDiagnostic();
  assert.notEqual(result.exitCode, 0, "lwc:slot-bind in shadow DOM must be rejected");
  assert.match(result.output, /lwc:slot-bind/i, "diagnostic must identify the invalid scoped-slot binding");
});
