import fs from "node:fs";
import { createRequire } from "node:module";

// Local transport for the exported native fixture. Each navigation resets the
// production refresh registries, matching native per-row page navigation.
const config = JSON.parse(fs.readFileSync(0, "utf8"));
const require = createRequire(import.meta.url);
const { chromium } = require(config.playwrightModule);
const browser = await chromium.launch({
  headless: true,
  ...(config.executablePath ? { executablePath: config.executablePath } : {}),
  timeout: 15000,
});
try {
  const values = {};
  const errors = {};
  const oneLine = message => String(message).replace(/https?:\/\/\S+/g, "<URL>").replace(/[\r\n\t]/g, " ").slice(0, 2000);
  // Rows spend most of their time in fixed, native-defined observation windows
  // (a 4 s bounded wait for an absent completion). Replay them on a few
  // independent browser contexts at once. Each worker owns one context and one
  // page and navigates it per row, so no state is shared between workers and the
  // rows of one worker run in the same order as before.
  const queue = [...config.ids];
  const worker = async () => {
    const context = await browser.newContext();
    try {
      const page = await context.newPage();
      let diagnostics = [];
      page.on("pageerror", error => diagnostics.push(oneLine(error.message)));
      const resultDOM = page.locator("[data-l16-host] [data-result]");
      const waitResult = async (id, stage) => {
        await resultDOM.waitFor({ state: "attached", timeout: 5000 });
        const deadline = Date.now() + 6000;
        while (Date.now() < deadline) {
          const text = await resultDOM.textContent({ timeout: 5000 });
          try {
            const value = JSON.parse(text);
            if (value.id === id && value.stage === stage) return value;
          } catch {
            // A render can still contain the preceding stage's text.
          }
          await new Promise(resolve => setTimeout(resolve, 20));
        }
        throw new Error("LOCAL_ROW_" + stage.toUpperCase() + "_TIMEOUT");
      };
      for (let id = queue.shift(); id !== undefined; id = queue.shift()) {
        diagnostics = [];
        try {
          const url = new URL(config.url);
          url.searchParams.set("case", id);
          await page.goto(url.href, { waitUntil: "domcontentloaded", timeout: 30000 });
          // Read the fixture's public result DOM on both routes. Never read private
          // component state or repair native/local observations in the transport.
          await waitResult(id, "ready");
          await page.locator("[data-l16-host] [data-run]").click({ timeout: 10000 });
          const result = await waitResult(id, "done");
          if (result.captureError || !result.observation) {
            errors[id] = oneLine(result.captureError || "ROW_ACKNOWLEDGEMENT_INVALID");
          } else {
            // Cleanup failures and any nulls remain part of the observed row.
            values[id] = result;
          }
        } catch (error) {
          errors[id] = oneLine(error.message) + (diagnostics.length ? "; " + diagnostics.join("; ") : "");
        }
      }
    } finally {
      await context.close();
    }
  };
  const workers = Math.max(1, Math.min(config.concurrency || 1, config.ids.length));
  await Promise.all(Array.from({ length: workers }, worker));
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
