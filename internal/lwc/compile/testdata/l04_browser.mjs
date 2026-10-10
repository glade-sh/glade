// Replay only owned local fixtures. Native answers live in l04_salesforce.json.
import fs from "node:fs";
import { launchBrowser, runRows } from "./browser_row_pool.mjs";

const config = JSON.parse(fs.readFileSync(0, "utf8"));
const browser = await launchBrowser(config);
// Every row has its own boundary, child bundle and trace module, mounts and
// unmounts itself, and settles its microtasks before the next step. Each worker
// context loads its own page and owns its console attribution state.
async function rowWorker(context) {
  const page = await context.newPage();
  let activeCase = null;
  let activeStep = null;
  const diagnostics = [];
  // Use the same case-owned diagnostic filter as the native observation route.
  const report = (type, message) => {
    if (activeCase && /(owned-|family-?l04|non-trackable)/i.test(message)) {
      diagnostics.push({
        case: activeCase,
        step: activeStep,
        type,
        message: message.replace(/https?:\/\/\S+/g, "[URL withheld]").slice(0, 2000),
      });
    }
  };
  page.on("console", message => {
    if (["warning", "error"].includes(message.type())) report(message.type(), message.text());
  });
  page.on("pageerror", error => report("pageerror", error.message));
  await page.goto(config.url, { waitUntil: "domcontentloaded", timeout: 30000 });
  await page.locator('[data-case="' + config.specs[0].id + '"]').waitFor({ state: "attached", timeout: 30000 });
  const settle = () => page.evaluate(() => new Promise(resolve => requestAnimationFrame(() => requestAnimationFrame(resolve))));
  return async spec => {
    const result = {};
    activeCase = spec.id;
    activeStep = 0;
    const row = page.locator('[data-case="' + spec.id + '"]');
    let retained = null;
    try {
      await row.waitFor({ state: "attached", timeout: 10000 });
      retained = await row.evaluateHandle(node => ({
        row: node,
        boundary: node.getRootNode().host || node.parentElement,
      }));
      const steps = [];
      let removedAt = null;
      const observe = async () => {
        await settle();
        const presence = await retained.evaluate(({ row, boundary }) => ({
          rowPresent: row.isConnected,
          boundaryPresent: boundary ? boundary.isConnected : null,
        }));
        if (!presence.rowPresent) {
          // Preserve actual detachment without inventing inaccessible DOM or traces.
          removedAt = "snapshot-" + activeStep;
          steps.push({
            step: activeStep,
            nativeFailure: true,
            ...presence,
            errors: diagnostics.filter(diagnostic => diagnostic.case === spec.id && diagnostic.step === activeStep && diagnostic.type !== "warning"),
          });
          return;
        }
        await row.locator("[data-snapshot]").click({ timeout: 10000 });
        await settle();
        const result = JSON.parse(await row.locator("[data-result]").textContent());
        steps.push({ step: activeStep, ...result });
      };
      await row.locator("[data-mount]").click({ timeout: 10000 });
      await observe();
      const controls = { advance: "[data-next]", mount: "[data-mount]", unmount: "[data-unmount]", "click-error": "[data-fire]" };
      for (let index = 0; index < spec.controls.length; index++) {
        if (removedAt !== null) break;
        activeStep = index + 1;
        if (!controls[spec.controls[index]]) throw new Error("Unknown owned control " + spec.controls[index]);
        await row.locator(controls[spec.controls[index]]).click({ timeout: 10000 });
        await observe();
      }
      result.value = { steps, console: diagnostics.filter(diagnostic => diagnostic.case === spec.id) };
      if (removedAt !== null) {
        result.value.status = "NATIVE_ROW_REMOVED";
        result.value.stage = removedAt;
      }
    } catch (error) {
      result.error = error.message;
    } finally {
      activeCase = null;
      activeStep = null;
      try {
        if (await row.count()) {
          await row.locator("[data-unmount]").click({ timeout: 10000 });
          await settle();
        }
      } catch (error) {
        delete result.value;
        result.error = "Local cleanup failed: " + error.message;
      }
      if (retained) await retained.dispose();
    }
    return result;
  };
}

try {
  const rows = await runRows(browser, config.specs, { setup: rowWorker, run: (observe, spec) => observe(spec) });
  const values = {};
  const errors = {};
  config.specs.forEach((spec, index) => {
    if ("value" in rows[index]) values[spec.id] = rows[index].value;
    if ("error" in rows[index]) errors[spec.id] = rows[index].error;
  });
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
