import assert from "node:assert/strict";
import path from "node:path";
import test from "node:test";
import { pathToFileURL } from "node:url";
import { startValidationMulticomponentServer } from "./validation-multicomponent-harness.mjs";

const playwrightPath = process.env.GLADE_LWC_PLAYWRIGHT_MODULE;
const playwrightModule = playwrightPath
  ? await import(pathToFileURL(path.extname(playwrightPath) ? playwrightPath : path.join(playwrightPath, "index.mjs")).href)
  : await import("playwright");
const { chromium } = playwrightModule;

async function readStateRow(page, index) {
  const row = page.locator('[data-testid="validation-state-table"] tbody tr').nth(index);
  await row.waitFor({ state: "visible" });
  return row.evaluate((element) => {
    const cells = [...element.cells].map((cell) => cell.textContent.trim());
    return {
      phase: cells[0],
      radioValue: cells[1],
      radioRequired: cells[2],
      radioCheck: cells[3],
      radioReport: cells[4],
      radioNativeValid: cells[5],
      radioVisibleInvalid: cells[6],
      radioVisibleMessage: cells[7],
      selectValue: cells[8],
      selectRequired: cells[9],
      selectCheck: cells[10],
      selectReport: cells[11],
      selectNativeValid: cells[12],
      selectVisibleInvalid: cells[13],
      selectVisibleMessage: cells[14],
    };
  });
}

function assertRequiredEmpty(row, component) {
  assert.equal(row[component + "Value"], "(empty)");
  assert.equal(row[component + "Required"], "true");
  assert.equal(row[component + "Check"], "false");
  assert.equal(row[component + "Report"], "false");
  assert.equal(row[component + "NativeValid"], "false");
  assert.equal(row[component + "VisibleInvalid"], "true");
  assert.notEqual(row[component + "VisibleMessage"], "(none)");
}

function assertValidSelection(row, component) {
  assert.equal(row[component + "Value"], "north");
  assert.equal(row[component + "Required"], "true");
  assert.equal(row[component + "Check"], "true");
  assert.equal(row[component + "Report"], "true");
  assert.equal(row[component + "NativeValid"], "true");
  assert.equal(row[component + "VisibleInvalid"], "false");
  assert.equal(row[component + "VisibleMessage"], "(none)");
}

test("one compiled API 67 consumer records radio-group and select validation states", async () => {
  const server = await startValidationMulticomponentServer({ port: 8942 });
  let browser;
  try {
    assert.equal(server.framework.declaredComponentApiVersion, 67);
    assert.deepEqual(
      [server.framework.compilerVersion, server.framework.engineDomVersion, server.framework.syntheticShadowVersion],
      ["9.4.3", "9.4.3", "9.4.3"],
      "framework package versions are local toolchain identity, not Salesforce API versions",
    );
    assert.equal(server.framework.componentReferenceBuildIdentity, "v262.0.0");
    assert.equal(server.framework.salesforceApiVersion, null);
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
    await page.waitForFunction(() => window.__validationMulticomponentMounted === true);
    assert.equal(await page.getByRole("button", { name: "Run family", exact: true }).count(), 1);
    assert.equal(
      await page.locator("lightning-select").evaluate((host) => host.shadowRoot?.querySelector("select option")?.value),
      "",
      "the select starts with an explicit empty-value option",
    );
    assert.deepEqual(
      await page.evaluate(() => {
        const root = document.querySelector("c-validation-multicomponent")?.shadowRoot;
        const radios = root?.querySelector("lightning-radio-group")?.shadowRoot?.querySelectorAll('input[type="radio"]') || [];
        const select = root?.querySelector("lightning-select")?.shadowRoot?.querySelector("select");
        return { radioGroup: [...radios].some((radio) => radio.required), select: Boolean(select?.required) };
      }),
      { radioGroup: true, select: true },
      "required must reach the native controls used by the shared validity methods",
    );

    await page.getByRole("button", { name: "Run family", exact: true }).click();
    const phases = ["required-empty", "valid-selection", "custom-error", "cleared-error"];
    await page.waitForFunction(() => {
      const consumer = document.querySelector("c-validation-multicomponent");
      const root = consumer?.shadowRoot;
      return root?.querySelector('[data-testid="phase"]')?.textContent.trim() === "complete" &&
        root.querySelectorAll('[data-testid="validation-state-table"] tbody tr').length === 4;
    });
    const tableRows = page.locator('[data-testid="validation-state-table"] tbody tr');
    assert.equal(await tableRows.count(), 4, "one click must append exactly four phase rows");
    const rows = [];
    for (let index = 0; index < 4; index++) rows.push(await readStateRow(page, index));

    assert.deepEqual(rows.map((row) => row.phase), phases, "the visible state table is append-only and ordered");
    assertRequiredEmpty(rows[0], "radio");
    assertRequiredEmpty(rows[0], "select");
    assertValidSelection(rows[1], "radio");
    assertValidSelection(rows[1], "select");
    for (const component of ["radio", "select"]) {
      assert.equal(rows[2][component + "Value"], "north");
      assert.equal(rows[2][component + "Check"], "false");
      assert.equal(rows[2][component + "Report"], "false");
      assert.equal(rows[2][component + "NativeValid"], "false");
      assert.equal(rows[2][component + "VisibleInvalid"], "true");
    }
    assert.equal(rows[2].radioVisibleMessage, "Radio custom error.");
    assert.equal(rows[2].selectVisibleMessage, "Select custom error.");
    assertValidSelection(rows[3], "radio");
    assertValidSelection(rows[3], "select");
    assert.deepEqual(pageErrors, []);
  } finally {
    await browser?.close();
    await server.close();
  }
});
