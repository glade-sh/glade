// Owned L17 local observation transport. Match the native three-page topology:
// ordered global-style rows on page 0, independent scripts on pages 1 and 2.
// Read public result DOM only. No expected answers enter this process.
import fs from "node:fs";
import { createRequire } from "node:module";

const config = JSON.parse(fs.readFileSync(0, "utf8"));
const require = createRequire(import.meta.url);
const { chromium } = require(config.playwrightModule);
const pageShards = require("./l17_page_shards.cjs");
const browser = await chromium.launch({
  headless: true,
  ...(config.executablePath ? { executablePath: config.executablePath } : {}),
  timeout: 15000,
});
const values = {}, errors = {};
const scrub = text => String(text).replace(/https?:\/\/[^\s"'<>]+/g, "[URL withheld]")
  .replace(/\/resource\/[^\s"'<>]+/g, "[RESOURCE URL]").slice(0, 1000);
try {
  const context = await browser.newContext({ viewport: { width: 1280, height: 900 }, ignoreHTTPSErrors: true });
  // The native transport shares a context, but each lane retains its own page,
  // loader registry, console and predecessor style state for the entire run.
  const firstPage = await context.newPage();
  await firstPage.goto(config.url, { waitUntil: "domcontentloaded", timeout: 30000 });
  await firstPage.locator('[data-case="' + config.ids[0] + '"]').waitFor({ state: "attached", timeout: 10000 });
  const host = await firstPage.evaluate(() => ({
    path: location.pathname,
    contentType: document.contentType,
    compatMode: document.compatMode,
    shell: document.body.dataset.gladeShell,
    bodyColor: getComputedStyle(document.body).color,
  }));
  if (host.shell !== "workbench" || host.compatMode !== "CSS1Compat") {
    throw new Error("L17 requires the production tab document in standards mode");
  }
  await pageShards(context, firstPage, config.ids, async (page, ids, lane, stop, scope) => {
    let active = null, prerequisite = null;
    const consoleRows = [];
    page.on("pageerror", error => consoleRows.push({ id: active, type: "pageerror", name: error.name, message: scrub(error.message) }));
    page.on("console", event => {
      if (["error", "warning"].includes(event.type())) {
        consoleRows.push({ id: active, type: event.type(), message: scrub(event.text()) });
      }
    });
    try {
      for (const id of ids) {
        if (stop.value) break;
        scope.row = id;
        if (prerequisite) {
          errors[id] = "CAPTURE_PREREQUISITE_UNAVAILABLE: " + prerequisite;
          continue;
        }
        try {
          active = id;
          const row = page.locator('[data-case="' + id + '"]');
          await row.waitFor({ state: "attached", timeout: 10000 });
          await row.locator("[data-run]").click({ timeout: 10000 });
          await row.locator("[data-result]").evaluate(node => new Promise((resolve, reject) => {
            if (node.textContent) { resolve(); return; }
            const observer = new MutationObserver(() => {
              if (node.textContent) { clearTimeout(timer); observer.disconnect(); resolve(); }
            });
            observer.observe(node, { childList: true, subtree: true, characterData: true });
            const timer = setTimeout(() => { observer.disconnect(); reject(new Error("owned resource row did not settle")); }, 20000);
          }));
          const observed = JSON.parse(await row.locator("[data-result]").textContent());
          if (observed.id !== id) throw new Error("owned row acknowledgement mismatch");
          observed.console = consoleRows.filter(row => row.id === id);
          values[id] = observed;
        } catch (error) {
          errors[id] = scrub(error.message);
          if (lane === 0) prerequisite = id;
        }
      }
    } catch (error) {
      for (const id of ids) if (!(id in values)) errors[id] = scrub(error.message);
    }
  }, false, id => config.pageAssignments[id]);
  await context.close();
  process.stdout.write(JSON.stringify({ values, errors, host }));
} finally {
  await browser.close();
}
