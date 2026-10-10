// Local observations only. Native answers never enter the browser configuration.
import fs from "node:fs";
import { createRequire } from "node:module";

const config = JSON.parse(fs.readFileSync(0, "utf8"));
const require = createRequire(import.meta.url);
const { chromium } = require(config.playwrightModule);
const browser = await chromium.launch({
  headless: true,
  ...(config.executablePath ? { executablePath: config.executablePath } : {}),
  timeout: 15000,
});
try {
  const page = await browser.newPage();
  const values = {};
  const errors = {};
  const diagnostics = [];
  page.on("pageerror", error => diagnostics.push(error.message));
  await page.goto(config.url, { waitUntil: "domcontentloaded", timeout: 30000 });
  // Fail a broken mount once instead of timing out every row in the table.
  try {
    await page.locator('[data-case="' + config.ids[0] + '"] [data-run]').waitFor({ state: "attached", timeout: 30000 });
  } catch (error) {
    throw new Error("L08 local mount: " + error.message + (diagnostics.length ? "; " + diagnostics.join("; ") : ""));
  }
  for (const id of config.ids) {
    const row = page.locator('[data-case="' + id + '"]');
    try {
      await row.locator("[data-run]").click({ timeout: 10000 });
      await row.locator("[data-result]").filter({ hasText: /./ }).waitFor({ state: "attached", timeout: 10000 });
      // Read the rendered text, retaining null and all string case/length.
      values[id] = JSON.parse(await row.locator("[data-result]").textContent());
    } catch (error) {
      errors[id] = error.message + (diagnostics.length ? "; " + diagnostics.join("; ") : "");
    }
  }
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
