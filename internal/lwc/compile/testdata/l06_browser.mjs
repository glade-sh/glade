// Observe only the owned local fixtures. Expected answers stay in the Go test.
// The frame acknowledgement and diagnostic filter match the native capture.
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
  const page = await browser.newPage();
  const diagnostics = [];
  let activeCase = null;
  let activeStep = null;
  const diagnostic = (type, message) => {
    if (!/\b(slot|ref|spread|manual|dynamic|component|render|dom)\b/i.test(message)) return;
    diagnostics.push({
      case: activeCase,
      step: activeStep,
      type,
      message: message.replace(/https?:\/\/\S+/g, "[URL withheld]").slice(0, 2000),
    });
  };
  page.on("console", message => {
    if (["warning", "error"].includes(message.type())) diagnostic(message.type(), message.text());
  });
  page.on("pageerror", error => diagnostic("pageerror", error.message));
  await page.goto(config.url, { waitUntil: "domcontentloaded", timeout: 30000 });
  const values = {};
  const errors = {};
  const waitForFrame = async (locator, step) => locator.evaluate((node, step) => new Promise((resolve, reject) => {
    let observer;
    let timer;
    const ready = () => {
      if (!node.textContent) return false;
      const value = JSON.parse(node.textContent);
      return value.requested === step &&
        (value.steps.some(s => s.step === step) || value.errors.some(e => e.step === step));
    };
    const finish = () => {
      if (!ready()) return;
      if (observer) observer.disconnect();
      clearTimeout(timer);
      resolve();
    };
    if (ready()) { resolve(); return; }
    observer = new MutationObserver(finish);
    observer.observe(node, { subtree: true, childList: true, characterData: true });
    timer = setTimeout(() => {
      observer.disconnect();
      reject(new Error("frame acknowledgement timed out"));
    }, 15000);
  }), step);
  for (const spec of config.specs) {
    activeCase = spec.id;
    activeStep = 0;
    const row = page.locator('[data-case="' + spec.id + '"]');
    const result = row.locator("[data-result]");
    try {
      await result.waitFor({ state: "attached", timeout: 10000 });
      await waitForFrame(result, 0);
      for (let step = 1; step < spec.frames; step++) {
        activeStep = step;
        await row.locator("[data-next]").click({ timeout: 10000 });
        await waitForFrame(result, step);
      }
      values[spec.id] = JSON.parse(await result.textContent());
    } catch (error) {
      // Keep failed rows in the Go report while continuing the full table.
      errors[spec.id] = error.message;
    }
  }
  for (const spec of config.specs) {
    if (Object.hasOwn(values, spec.id)) {
      values[spec.id].console = diagnostics.filter(d => d.case === spec.id);
    }
  }
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
