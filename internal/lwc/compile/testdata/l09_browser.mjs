// Replay native controls in source order. Only local connection settings and
// row IDs enter this observer; every answer comes from the compiled fixture DOM.
import fs from "node:fs";
import { createRequire } from "node:module";

const config = JSON.parse(fs.readFileSync(0, "utf8"));
const { chromium } = createRequire(import.meta.url)(config.playwrightModule);
const browser = await chromium.launch({
  headless: true,
  ...(config.executablePath ? { executablePath: config.executablePath } : {}),
  timeout: 15000,
});
try {
  const values = {}, errors = {};
  const page = await browser.newPage();
  const pageErrors = [];
  page.on("pageerror", error => pageErrors.push(error.name + ": " + error.message));
  let failure = "";
  try {
    await page.goto(config.url, { waitUntil: "domcontentloaded", timeout: 30000 });
    await page.locator('[data-host="L09"]').waitFor({ state: "attached", timeout: 30000 });
    const startup = await page.evaluate(() => window.__l09Error);
    if (startup) throw new Error(startup.name + ": " + startup.message);
  } catch (error) {
    failure = "local page initialization: " + error.message;
    const startup = await page.evaluate(() => window.__l09Error).catch(() => null);
    if (startup) failure += "; " + startup.name + ": " + startup.message;
    if (pageErrors.length) failure += "; " + pageErrors.slice(0, 4).join("; ");
  }
  for (const id of config.ids) {
    if (failure) {
      errors[id] = failure;
      continue;
    }
    try {
      await page.locator('[data-id="' + id + '"]').click({ timeout: 30000 });
      const result = page.locator('[data-result="L09"]');
      await result.evaluate((node, id) => new Promise((resolve, reject) => {
        const complete = () => {
          try { return JSON.parse(node.textContent).id === id; }
          catch { return false; }
        };
        if (complete()) { resolve(); return; }
        const observer = new MutationObserver(() => {
          if (complete()) { observer.disconnect(); clearTimeout(timer); resolve(); }
        });
        // The native fixture owns its 4000/30000ms bounded observations.
        // An observer timeout is an error, never a replacement null answer.
        const timer = setTimeout(() => {
          observer.disconnect(); reject(new Error("owned acknowledgement timeout"));
        }, 120000);
        observer.observe(node, { subtree: true, childList: true, characterData: true });
      }), id);
      const answer = JSON.parse(await result.textContent());
      if (answer.id !== id || !Object.hasOwn(answer, "observation") ||
          !Array.isArray(answer.cleanupErrors) || answer.captureError) {
        throw new Error("invalid owned acknowledgement");
      }
      if (answer.cleanupErrors.length) {
        errors[id] = "owned record cleanup failed: " + JSON.stringify(answer.cleanupErrors);
        continue;
      }
      // Includes the raw null when observed; missing observation is checked above.
      values[id] = answer.observation;
    } catch (error) {
      errors[id] = error.message;
      // Preserve the native page's cache history. Do not recreate a page after
      // a failed acknowledgement and pretend dependent rows were observed.
      failure = "local page stopped at " + id + ": " + error.message;
    }
  }
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
