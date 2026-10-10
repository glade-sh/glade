// Observe the exported native fixture through the local product. Inputs contain
// row IDs and browser connection settings; no oracle answers enter this runner.
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
  const mutations = new Set(["createList", "updateList", "deleteList", "updateListPreferences"]);
  const reads = config.specs.filter(spec => !mutations.has(spec.adapter));
  const writes = config.specs.filter(spec => mutations.has(spec.adapter));
  async function observe(specs) {
    const page = await context.newPage();
    const pageErrors = [];
    page.on("pageerror", error => pageErrors.push(error.name + ": " + error.message));
    page.on("console", async message => {
      if (message.type() !== "error") return;
      for (const handle of message.args()) {
        const cause = await handle.evaluate(value => {
          const parts = [];
          for (let error = value; error && typeof error === "object" && parts.length < 4; error = error.cause) {
            if (typeof error.message === "string") parts.push((error.name || "Error") + ": " + error.message);
          }
          return parts.join("; caused by ");
        }).catch(() => "");
        if (cause) pageErrors.push(cause);
      }
    });
    let initializationFailure = "";
    for (const spec of specs) {
      if (initializationFailure) {
        errors[spec.id] = initializationFailure;
        continue;
      }
      pageErrors.length = 0;
      let initialized = false;
      try {
        const url = new URL(config.url);
        url.searchParams.set("c__case", spec.id);
        await page.goto(url.href, { waitUntil: "domcontentloaded", timeout: 30000 });
        // Preserve the native route-per-row page and bounded 30000ms reads.
        // Playwright locators cross LWC shadow roots without modifying the page.
        const result = page.locator("[data-l11-host] [data-result]");
        await result.waitFor({ state: "attached", timeout: 30000 });
        const startup = await page.evaluate(() => window.__l11Error);
        if (startup) throw new Error(startup.name + ": " + startup.message);
        const ready = JSON.parse(await result.textContent());
        if (ready.id !== spec.id || ready.stage !== "ready") {
          throw new Error("invalid owned ready acknowledgement");
        }
        initialized = true;
        await page.locator("[data-l11-host] [data-run]").click({ timeout: 30000 });
        const timeout = ["createList", "updateList", "deleteList"].includes(spec.adapter)
          ? 240000 : spec.adapter === "updateListPreferences" ? 135000 : 95000;
        await result.evaluate((node, { id, timeout }) => new Promise((resolve, reject) => {
          const complete = () => {
            try {
              const answer = JSON.parse(node.textContent);
              return answer.id === id && ["done", "fixture-error"].includes(answer.stage);
            } catch { return false; }
          };
          if (complete()) { resolve(); return; }
          const observer = new MutationObserver(() => {
            if (complete()) { observer.disconnect(); clearTimeout(timer); resolve(); }
          });
          // This timeout is an observation error, never a synthetic null or
          // noDataOrErrorWithinMs answer. Those belong to the native fixture.
          const timer = setTimeout(() => {
            observer.disconnect(); reject(new Error("owned acknowledgement timeout"));
          }, timeout);
          observer.observe(node, { subtree: true, childList: true, characterData: true });
        }), { id: spec.id, timeout });
        const answer = JSON.parse(await result.textContent());
        if (answer.id !== spec.id || answer.stage !== "done" ||
            !Object.hasOwn(answer, "observation")) {
          throw new Error("invalid owned acknowledgement: " + JSON.stringify(answer));
        }
        // Preserve the raw null if the fixture renders one.
        values[spec.id] = answer.observation;
      } catch (error) {
        let failure = error.message;
        const startup = await page.evaluate(() => window.__l11Error).catch(() => null);
        if (startup) failure += "; " + startup.name + ": " + startup.message;
        if (pageErrors.length) failure += "; " + pageErrors.slice(0, 4).join("; ");
        errors[spec.id] = failure;
        if (!initialized) initializationFailure = "local page initialization: " + failure;
      }
    }
    await page.close();
  }
  // Match the captured native schedule: finish every read before serial
  // mutations can delete fixture views or change preference snapshots.
  for (const [specs, concurrency] of [[reads, 4], [writes, 1]]) {
    const groups = Array.from({ length: Math.min(concurrency, specs.length) }, () => []);
    specs.forEach((spec, index) => groups[index % groups.length].push(spec));
    await Promise.all(groups.map(observe));
  }
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
