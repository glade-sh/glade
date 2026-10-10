// Replay the native page navigation and owned controls. No oracle answers enter here.
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
  const context = await browser.newContext();
  const page = await context.newPage();
  let pageErrors = [];
  page.on("pageerror", error => pageErrors.push(error.name + ": " + error.message));
  let startupFailure = "";
  for (const id of config.ids) {
    if (startupFailure) {
      errors[id] = startupFailure;
      continue;
    }
    pageErrors = [];
    let initialized = false;
    try {
      const url = new URL(config.url);
      url.searchParams.set("c__case", id);
      await page.goto(url.href, { waitUntil: "domcontentloaded", timeout: 30000 });
      const result = page.locator("[data-l12-host] [data-result]");
      await result.waitFor({ state: "attached", timeout: 30000 });
      const startup = await page.evaluate(() => window.__l12Error);
      if (startup) throw new Error(startup.name + ": " + startup.message);
      const waitStage = stage => result.evaluate((node, { id, stage }) => new Promise((resolve, reject) => {
        const complete = () => {
          try {
            const answer = JSON.parse(node.textContent);
            return answer.id === id && answer.stage === stage;
          } catch { return false; }
        };
        if (complete()) { resolve(); return; }
        const observer = new MutationObserver(() => {
          if (complete()) { observer.disconnect(); clearTimeout(timer); resolve(); }
        });
        const timer = setTimeout(() => {
          observer.disconnect(); reject(new Error("owned acknowledgement timeout: " + stage));
        }, 60000);
        observer.observe(node, { subtree: true, childList: true, characterData: true });
      }), { id, stage });
      await waitStage("ready");
      initialized = true;
      await page.locator("[data-l12-host] [data-run]").click({ timeout: 30000 });
      // Preserve the native 4000ms windows and 200ms settles in the owned source.
      await waitStage("done");
      const answer = JSON.parse(await result.textContent());
      if (answer.captureError) {
        errors[id] = "owned fixture/host failure: " + JSON.stringify(answer.captureError);
      } else if (answer.id !== id || answer.stage !== "done" || !Array.isArray(answer.trace)) {
        throw new Error("invalid owned acknowledgement");
      } else {
        values[id] = answer;
      }
    } catch (error) {
      let failure = error.message;
      const startup = await page.evaluate(() => window.__l12Error).catch(() => null);
      if (startup) failure += "; " + startup.name + ": " + startup.message;
      if (pageErrors.length) failure += "; " + pageErrors.slice(0, 4).join("; ");
      errors[id] = failure;
      // Every row uses the same bundle. A startup failure leaves its remaining
      // DOM rows unobserved; it must never become a native no-data answer.
      if (!initialized) startupFailure = "local page initialization: " + failure;
    }
  }
  await context.close();
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
