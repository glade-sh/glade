// Observe the exported owned component through Glade's real local tab and Visualforce hosts.
// No Salesforce transport, oracle answers, fake wire values or navigation shim.
import fs from "node:fs";
import { launchBrowser, runRows } from "./browser_row_pool.mjs";

const config = JSON.parse(fs.readFileSync(0, "utf8"));
const browser = await launchBrowser(config);

async function observation(page, row, stage) {
  const result = page.locator(row.selector + " [data-result]");
  const deadline = Date.now() + 5000;
  while (Date.now() < deadline) {
    try {
      // Reacquire the DOM text on every attempt so a real document navigation
      // cannot leave the observer attached to the previous result element.
      const text = await result.textContent({ timeout: 500 });
      const value = JSON.parse(text);
      if (value.id === row.id && value.stage === stage) return text;
    } catch { /* A navigation can temporarily detach the result. */ }
    await page.waitForTimeout(50);
  }
  throw new Error("ROW_" + stage.toUpperCase() + "_TIMEOUT");
}

// Rows read only host state, never write the org, and each row still gets its
// own fresh context, so the pool's worker context is not used for rows.
async function observeRow(row) {
  // Fresh history and storage for each row. Navigate is acknowledged only
  // by the component's real CurrentPageReference wire, as in the native run.
  const context = await browser.newContext();
  const page = await context.newPage();
  const errors = [];
  page.on("pageerror", error => errors.push(error.name + ": " + error.message));
  await context.route("**/*", route => {
    const url = new URL(route.request().url());
    return url.origin === config.origin ? route.continue() : route.abort();
  });
  try {
    const response = await page.goto(row.url, { waitUntil: "domcontentloaded", timeout: 15000 });
    if (!response || response.status() !== 200) {
      // A host/compiler failure is not a navigation observation. Preserve
      // the response diagnostic so capture mode exposes its actual cause.
      const body = response ? await response.text() : "";
      throw new Error("HOST_HTTP_" + (response?.status() ?? "NO_RESPONSE") +
        (body ? "|body=" + JSON.stringify(body) : ""));
    }
    await observation(page, row, "ready");
    await page.locator(row.selector + " [data-run]").click({ timeout: 5000 });
    // Reacquire after Navigate may have loaded a new document. Do not treat
    // a returned call or unchanged ready result as successful navigation.
    return "BROWSER|" + await observation(page, row, "done");
  } catch (error) {
    const last = await page.locator(row.selector + " [data-result]").first().textContent({ timeout: 1000 }).catch(() => "");
    return "BROWSER_ERROR|" + error.message +
      (last ? "|last=" + last : "") + (errors.length ? "|pageerrors=" + JSON.stringify(errors) : "");
  } finally {
    await context.close();
  }
}

try {
  const results = await runRows(browser, config.rows, { setup: () => null, run: (_, row) => observeRow(row) });
  const values = {};
  config.rows.forEach((row, index) => { values[row.id] = results[index]; });
  process.stdout.write(JSON.stringify({ values }));
} finally {
  await browser.close();
}
