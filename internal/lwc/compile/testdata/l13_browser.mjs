// Observe compiled local fixtures only. The browser never receives oracle answers.
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
  const page = await browser.newPage();
  const values = {};
  const errors = {};
  for (const row of config.rows) {
    try {
      await page.goto(row.url, { waitUntil: "domcontentloaded", timeout: 15000 });
      const result = page.locator("[data-result]");
      await Promise.race([
        result.waitFor({ state: "attached", timeout: 5000 }),
        page.waitForFunction(() => window.__l13Error, null, { timeout: 5000 }),
      ]);
      const error = await page.evaluate(() => window.__l13Error);
      if (error) {
        // Only an actual explicit context error satisfies the local boundary.
        values[row.id] = error.code === "EXPERIENCE_BUILDER_CONTEXT_REQUIRED" ||
          error.message === "EXPERIENCE_BUILDER_CONTEXT_REQUIRED"
          ? "BOUNDARY_ERROR|EXPERIENCE_BUILDER_CONTEXT_REQUIRED"
          : "RUNTIME_ERROR|" + error.name + "|" + error.message;
      } else {
        // Playwright locators observe the real result across open shadow roots.
        await result.evaluate(node => new Promise((resolve, reject) => {
          if (node.textContent) { resolve(); return; }
          const observer = new MutationObserver(() => {
            if (node.textContent) { observer.disconnect(); clearTimeout(timer); resolve(); }
          });
          const timer = setTimeout(() => { observer.disconnect(); reject(new Error("Empty local result")); }, 5000);
          observer.observe(node, { subtree: true, childList: true, characterData: true });
        }));
        values[row.id] = "DOM|" + await result.textContent();
      }
    } catch (error) {
      errors[row.id] = error.message;
    }
  }
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
