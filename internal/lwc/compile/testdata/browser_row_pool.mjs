// Shared row pool for local browser drivers. Each worker owns one fresh
// browser context, runs the driver's per-context setup once, and takes
// independent rows from one queue. Results keep the original row order.
import { createRequire } from "node:module";

export const defaultRowConcurrency = 6;

// GLADE_LWC_BROWSER_CONCURRENCY overrides the worker count; 1 runs rows serially.
export function rowConcurrency(value = process.env.GLADE_LWC_BROWSER_CONCURRENCY) {
  if (value === undefined || value === "") return defaultRowConcurrency;
  const count = Number(value);
  if (!Number.isInteger(count) || count < 1) {
    throw new Error("GLADE_LWC_BROWSER_CONCURRENCY must be a positive integer: " + value);
  }
  return count;
}

export async function launchBrowser(config) {
  const { chromium } = createRequire(import.meta.url)(config.playwrightModule);
  return chromium.launch({
    headless: true,
    ...(config.executablePath ? { executablePath: config.executablePath } : {}),
    timeout: 15000,
  });
}

// setup(context) returns the worker state; run(state, row, index) returns the
// row result. A throw from either stops the queue and rejects after every
// worker has closed its context.
export async function runRows(browser, rows, { setup, run, concurrency = rowConcurrency() }) {
  const results = new Array(rows.length);
  let next = 0;
  let failed = false, failure;
  const fail = error => { if (!failed) { failed = true; failure = error; } };
  async function worker() {
    let context;
    try {
      context = await browser.newContext();
      const state = await setup(context);
      while (!failed && next < rows.length) {
        const index = next++;
        results[index] = await run(state, rows[index], index);
      }
    } catch (error) {
      fail(error);
    } finally {
      if (context) await context.close().catch(fail);
    }
  }
  const workers = Math.min(concurrency, rows.length);
  await Promise.all(Array.from({ length: workers }, worker));
  if (failed) throw failure;
  return results;
}
