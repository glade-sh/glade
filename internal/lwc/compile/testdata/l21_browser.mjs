// Local replay of the captured L21 fixture and controls. No oracle answers enter this process.
import fs from "node:fs";
import { createRequire } from "node:module";
import { pageCount } from "./browser_pages.mjs";
import { observeDOM, activateControl } from "./l21_dom_observer.mjs";

const config = JSON.parse(fs.readFileSync(0, "utf8"));
const { chromium } = createRequire(import.meta.url)(config.playwrightModule);
const browser = await chromium.launch({
  headless: true,
  ...(config.executablePath ? { executablePath: config.executablePath } : {}),
  timeout: 15000,
});
try {
  const context = await browser.newContext({ locale: "en-US", timezoneId: "America/Los_Angeles" });
  const values = {}, errors = {};
  // Each ?c__case navigation rebuilds page/module/component caches and fixture state.
  // Shared context cookies/storage/service workers are unused; the server serves
  // immutable assets and CurrentPageReference, with no record/LDS mutation calls.
  // Clicks, focus, rich-text assignments and tab removal affect only that row's DOM;
  // all within-row steps stay serial. Every page shares the captured locale/timezone.
  const count = pageCount(1, 4, config.rows.length);
  const pages = [];
  // L21 startup failures abort the driver; navigation failures stay row-local.
  for (let index = 0; index < count; index++) pages.push(await context.newPage());
  async function waitForStage(page, id, stage) {
    await page.waitForFunction(({ id, stage }) => {
      if (window.__l21Error) return true;
      function find(root) {
        const result = root.querySelector("[data-result]");
        if (result) {
          try {
            const value = JSON.parse(result.textContent);
            if (value.id === id && value.stage === stage) return true;
          } catch (_) {}
        }
        for (const node of root.querySelectorAll("*")) {
          if (node.shadowRoot && find(node.shadowRoot)) return true;
        }
        return false;
      }
      return find(document);
    }, { id, stage }, { timeout: 5000 });
    const error = await page.evaluate(() => window.__l21Error);
    if (error) throw new Error(error.name + ": " + error.message);
  }
  async function runRows(page, rows) {
    for (const row of rows) {
      let stage = "navigate";
      try {
        // Each full navigation resets component, event, module and fixture state.
        const url = new URL(config.url);
        url.searchParams.set("c__case", row.id);
        await page.goto(url.href, { waitUntil: "domcontentloaded", timeout: 15000 });
        stage = "ready";
        await waitForStage(page, row.id, stage);
        const host = page.locator("[data-l21-host]");
        stage = "assign";
        await host.locator("[data-run]").click({ timeout: 5000 });
        stage = "assigned";
        await waitForStage(page, row.id, stage);
        const before = await host.evaluate(observeDOM, config.observationAttributes || []);
        const actions = [];
        for (const step of row.actions || []) {
          stage = "action";
          actions.push(await host.evaluate(activateControl, { step, includeTargetText: config.actionTargetText }));
          await host.evaluate(async () => {
            await new Promise(resolve => setTimeout(resolve, 500));
            return null;
          });
        }
        stage = "finish";
        await host.locator("[data-finish]").click({ timeout: 5000 });
        stage = "done";
        await waitForStage(page, row.id, stage);
        const value = JSON.parse(await host.locator("[data-result]").textContent());
        if (value.id !== row.id || value.stage !== "done") throw new Error("wrong local row acknowledgement");
        values[row.id] = {
          ...value,
          renderedBefore: before,
          renderedAfter: await host.evaluate(observeDOM, config.observationAttributes || []),
          actions,
        };
      } catch (error) {
        errors[row.id] = stage + ": " + error.message;
      }
    }
  }
  await Promise.all(pages.map((page, index) =>
    runRows(page, config.rows.filter((_, rowIndex) => rowIndex % count === index))));
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
