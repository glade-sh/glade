// Local replay of owned L05 controls. Expected native rows are never supplied.
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
  await page.goto(config.url, { waitUntil: "domcontentloaded", timeout: 30000 });
  const values = {}, errors = {};
  for (const id of config.ids) {
    try {
      const row = page.locator('[data-case="' + id + '"]');
      await row.locator("[data-run]").click({ timeout: 30000 });
      const result = row.locator("[data-result]");
      await result.evaluate(node => new Promise((resolve, reject) => {
        if (node.textContent) { resolve(); return; }
        const observer = new MutationObserver(() => {
          if (node.textContent) { observer.disconnect(); clearTimeout(timer); resolve(); }
        });
        observer.observe(node, { subtree: true, childList: true, characterData: true });
        const timer = setTimeout(() => {
          observer.disconnect(); reject(new Error("owned acknowledgement timeout"));
        }, 15000);
      }));
      values[id] = JSON.parse(await result.textContent());
    } catch (error) {
      errors[id] = error.message;
    }
  }
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
