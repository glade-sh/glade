// Replay the captured component controls. Native answers never enter this observer.
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
  // These metadata rows have no record mutations or cache-history prerequisites.
  // Each navigation supplies the native c__case state to CurrentPageReference.
  const groups = [[], [], []];
  config.specs.forEach((spec, index) => groups[index % groups.length].push(spec));
  const context = await browser.newContext();
  await Promise.all(groups.map(async specs => {
    const page = await context.newPage();
    let pageErrors = [];
    page.on("pageerror", error => pageErrors.push(error.name + ": " + error.message));
    for (const spec of specs) {
      pageErrors = [];
      try {
        const url = new URL(config.url);
        url.searchParams.set("c__case", spec.id);
        await page.goto(url.href, { waitUntil: "domcontentloaded", timeout: 30000 });
        const result = page.locator("[data-l10-host] [data-result]");
        // Playwright locators cross the component's shadow root.
        await result.waitFor({ state: "attached", timeout: 30000 });
        const startup = await page.evaluate(() => window.__l10Error);
        if (startup) throw new Error(startup.name + ": " + startup.message);
        const ready = JSON.parse(await result.textContent());
        if (ready.id !== spec.id || ready.stage !== "ready") {
          throw new Error("invalid owned ready acknowledgement");
        }
        await page.locator("[data-l10-host] [data-run]").click({ timeout: 30000 });
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
          // The component retains its captured 30000ms no-data/error window.
          // An observer timeout is missing local output, never that native answer.
          const timer = setTimeout(() => {
            observer.disconnect(); reject(new Error("owned acknowledgement timeout"));
          }, 45000);
          observer.observe(node, { subtree: true, childList: true, characterData: true });
        }), spec.id);
        const answer = JSON.parse(await result.textContent());
        if (answer.id !== spec.id || answer.stage !== "done" ||
            !answer.observation || typeof answer.observation !== "object" ||
            Array.isArray(answer.observation)) {
          throw new Error("invalid owned done acknowledgement");
        }
        values[spec.id] = answer.observation;
      } catch (error) {
        const startup = await page.evaluate(() => window.__l10Error).catch(() => null);
        errors[spec.id] = error.message +
          (startup ? "; " + startup.name + ": " + startup.message : "") +
          (pageErrors.length ? "; " + pageErrors.slice(0, 4).join("; ") : "");
      }
    }
    await page.close();
  }));
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
