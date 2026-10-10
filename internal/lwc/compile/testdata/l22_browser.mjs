// Replay owned native controls on local product modules. No native answers enter
// this process. Fresh navigation isolates outstanding promises and toast stacks.
import fs from "node:fs";
import { createRequire } from "node:module";
import observeDOM from "./l22_dom_observer.mjs";
import overlayAction from "./l22_overlay_actions.mjs";

const config = JSON.parse(fs.readFileSync(0, "utf8"));
const { chromium } = createRequire(import.meta.url)(config.playwrightModule);
const browser = await chromium.launch({
  headless: true,
  timeout: 15000,
  ...(config.executablePath ? { executablePath: config.executablePath } : {}),
});

// Match json.dumps(sort_keys=True, separators=(',', ':'), ensure_ascii=True).
// Preserve every key, array position, string code unit and raw JSON null.
function browserText(value) {
  function sorted(v) {
    if (Array.isArray(v)) return v.map(sorted);
    if (v !== null && typeof v === "object") {
      return Object.fromEntries(Object.keys(v).sort().map(key => [key, sorted(v[key])]));
    }
    return v;
  }
  return "BROWSER|" + JSON.stringify(sorted(value)).replace(/[\u007f-\uffff]/g,
    ch => "\\u" + ch.charCodeAt(0).toString(16).padStart(4, "0"));
}

async function waitRow(page, id, stages) {
  await page.waitForFunction(({ id, stages }) => {
    const seen = new Set();
    function find(root) {
      if (!root || seen.has(root)) return;
      seen.add(root);
      for (const node of root.querySelectorAll("[data-result]")) {
        try {
          const value = JSON.parse(node.textContent);
          if (value.id === id && stages.includes(value.stage)) return value;
        } catch {}
      }
      for (const node of root.querySelectorAll("*")) {
        const value = find(node.shadowRoot);
        if (value) return value;
      }
    }
    return !!find(document);
  }, { id, stages }, { timeout: 15000 });
  return JSON.parse(await page.locator("[data-l22-host] [data-result]").textContent());
}

try {
  const values = {}, errors = {};
  let next = 0;
  async function worker() {
    const page = await browser.newPage();
    try {
      while (next < config.specs.length) {
        const spec = config.specs[next++];
        try {
          await page.goto(config.url + "?c__case=" + encodeURIComponent(spec.id), {
            waitUntil: "domcontentloaded", timeout: 30000,
          });
          let value = await waitRow(page, spec.id, ["ready", "done"]);
          if (value.stage !== "done") {
            await page.locator("[data-l22-host] [data-run]").click({ timeout: 15000 });
            value = await waitRow(page, spec.id, ["interaction-ready", "done"]);
            if (value.stage !== "done") {
              const snapshots = [await page.evaluate(observeDOM)];
              const actions = [(await overlayAction(page, spec)).observation];
              if (spec.operation === "open_twice") {
                actions.push((await overlayAction(page, spec)).observation);
              }
              if (spec.operation === "reopen" && actions[0].performed) {
                await page.locator("[data-l22-host] [data-reopen]").evaluate(node => node.click());
                actions.push((await overlayAction(page, spec)).observation);
              }
              snapshots.push(await page.evaluate(observeDOM));
              await page.locator("[data-l22-host] [data-finish]").evaluate(node => node.click());
              value = await waitRow(page, spec.id, ["done"]);
              if (!value.observation || typeof value.observation !== "object") {
                throw new Error("invalid owned row acknowledgement");
              }
              value.observation.browserActions = actions;
              value.observation.domCheckpoints = snapshots;
              value.observation.domAfterSettlement = await page.evaluate(observeDOM);
            }
          }
          values[spec.id] = browserText(value);
        } catch (error) {
          if (page.isClosed() || !browser.isConnected()) throw error;
          errors[spec.id] = error.message;
        }
      }
    } finally {
      await page.close();
    }
  }
  // Independent pages retain each row's complete observation windows.
  await Promise.all(Array.from({ length: config.concurrency }, () => worker()));
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
