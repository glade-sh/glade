// Replay owned native controls against the local product. No oracle answers enter here.
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
  const groups = [[], [], []];
  let recordIndex = 0;
  for (const spec of config.specs) {
    // This is the native page assignment; Apex cache prerequisites stay in order.
    const index = spec.adapter === "apex" ? 0 : 1 + recordIndex++ % 2;
    groups[index].push(spec);
  }
  const context = await browser.newContext();
  await Promise.all(groups.map(async specs => {
    const page = await context.newPage();
    const pageErrors = [];
    page.on("pageerror", error => pageErrors.push(error.name + ": " + error.message));
    let failure = "";
    try {
      await page.goto(config.url, { waitUntil: "domcontentloaded", timeout: 30000 });
      // Component-owned nodes live inside LWC shadow roots. Playwright's
      // locators cross those roots; document.querySelector does not.
      await page.locator('[data-host="L07"]').waitFor({ state: "attached", timeout: 30000 });
      const startup = await page.evaluate(() => window.__l07Error);
      if (startup) throw new Error(startup.name + ": " + startup.message);
    } catch (error) {
      failure = "local page initialization: " + error.message;
      const startup = await page.evaluate(() => window.__l07Error).catch(() => null);
      if (startup) failure += "; " + startup.name + ": " + startup.message;
      if (pageErrors.length) failure += "; " + pageErrors.slice(0, 4).join("; ");
    }
    for (const spec of specs) {
      if (failure) {
        errors[spec.id] = failure;
        continue;
      }
      try {
        await page.locator('[data-id="' + spec.id + '"]').click({ timeout: 30000 });
        // Keep the fixture's actual 4000ms initial/settled windows. A local timeout
        // is a missing observation, never a native no-data answer.
        const result = page.locator('[data-result="L07"]');
        await result.evaluate((node, id) => new Promise((resolve, reject) => {
          const complete = () => {
            try {
              const answer = JSON.parse(node.textContent);
              return answer.id === id && answer.stage === "done";
            } catch { return false; }
          };
          if (complete()) { resolve(); return; }
          const observer = new MutationObserver(() => {
            if (complete()) { observer.disconnect(); clearTimeout(timer); resolve(); }
          });
          const timer = setTimeout(() => {
            observer.disconnect(); reject(new Error("owned acknowledgement timeout"));
          }, 120000);
          observer.observe(node, { subtree: true, childList: true, characterData: true });
        }), spec.id);
        const answer = JSON.parse(await result.textContent());
        if (answer.id !== spec.id || answer.stage !== "done" || !Array.isArray(answer.cleanupErrors)) {
          throw new Error("invalid owned acknowledgement");
        }
        values[spec.id] = answer;
      } catch (error) {
        errors[spec.id] = error.message;
        // Resuming on a fresh page would change the cache history. Mark every
        // dependent row as unobserved instead of manufacturing a warm cache.
        failure = "local page stopped at " + spec.id + ": " + error.message;
      }
    }
    await page.close();
  }));
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
