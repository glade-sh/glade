import fs from "node:fs";
import { createRequire } from "node:module";

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
  // Rows spend most of their time in fixed observation pauses inside the native
  // fixture. Replay them on a few independent browser contexts at once. Each
  // worker owns one context and one page and reloads it per row, so no state is
  // shared between workers and a worker's rows keep their original order.
  const queue = [...config.ids];
  const worker = async () => {
    const context = await browser.newContext();
    try {
      const page = await context.newPage();
      let diagnostics = [];
      page.on("pageerror", error => diagnostics.push(error.message));
      page.on("console", message => {
        if (message.type() === "error") diagnostics.push(message.text());
      });
      // Reload the complete captured page per case, as on the native route, so
      // contexts, subscriptions and component snapshots cannot leak between rows.
      for (let id = queue.shift(); id !== undefined; id = queue.shift()) {
        diagnostics = [];
        let retained = null;
        let stage = "navigate";
        try {
          process.stderr.write("L15 row " + id + " starting\n");
          const url = new URL(config.url);
          url.searchParams.set("c__case", id);
          await page.goto(url.href, { waitUntil: "domcontentloaded", timeout: 30000 });
          const result = page.locator("[data-l15-host] [data-result]");
          stage = "ready";
          retained = await result.elementHandle({ timeout: 15000 });
          // Document queries in the page's main world exclude synthetic-shadow
          // descendants. Poll the element located by Playwright instead.
          const waitStage = async expectedStage => {
            const observation = await page.waitForFunction(({ node, expectedID, expectedStage }) => {
              if (!node || !node.isConnected) return false;
              try {
                const value = JSON.parse(node.textContent);
                return value.id === expectedID && value.stage === expectedStage;
              } catch {
                return false;
              }
            }, { node: retained, expectedID: id, expectedStage }, { timeout: 15000 });
            await observation.dispose();
          };
          await waitStage("ready");
          stage = "click";
          await page.locator("[data-l15-host] [data-run]").click({ timeout: 10000 });
          stage = "done";
          await waitStage("done");
          // Preserve all observed values, including captureError and raw null.
          // No native answers or product substitutes enter the browser process.
          values[id] = JSON.parse(await result.textContent());
          process.stderr.write("L15 row " + id + " observed\n");
        } catch (error) {
          errors[id] = [stage + ": " + error.message, ...diagnostics].join("; ");
          process.stderr.write("L15 row " + id + " failed " + errors[id] + "\n");
        } finally {
          if (retained) await retained.dispose();
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
