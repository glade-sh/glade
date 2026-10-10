import fs from "node:fs";
import path from "node:path";
import { pathToFileURL } from "node:url";

// Observe the same owned components and controls as the native capture. Only
// object-key ordering is canonicalized; values, nulls and node tokens stay exact.
function canonical(value) {
  if (Array.isArray(value)) return value.map(canonical);
  if (value !== null && typeof value === "object") {
    return Object.fromEntries(Object.keys(value).sort().map(key => [key, canonical(value[key])]));
  }
  return value;
}

let browser;
try {
  const config = JSON.parse(fs.readFileSync(0, "utf8"));
  const { chromium } = await import(pathToFileURL(path.join(config.playwrightModule, "index.mjs")).href);
  browser = await chromium.launch({
    headless: true,
    ...(config.executablePath ? { executablePath: config.executablePath } : {}),
    timeout: 15000,
  });
  const page = await browser.newPage();
  const diagnostics = [];
  const pageErrors = [];
  const startupErrors = [];
  let activeCase = null;
  let activeStep = null;
  const diagnostic = (type, message) => {
    if (!/\b(key|iterator|iteration|for:each)\b/i.test(message)) return;
    diagnostics.push({
      case: activeCase, step: activeStep, type,
      message: message.replace(/https?:\/\/\S+/g, "[URL withheld]").slice(0, 2000),
    });
  };
  page.on("console", message => {
    if (["warning", "error"].includes(message.type())) {
      if (activeCase === null && startupErrors.length < 8) startupErrors.push(message.text());
      diagnostic(message.type(), message.text());
    }
  });
  page.on("pageerror", error => {
    pageErrors.push(error.message);
    diagnostic("pageerror", error.message);
  });
  await page.goto(config.url, { waitUntil: "domcontentloaded", timeout: 30000 });
  try {
    await page.locator("[data-case]").first().waitFor({ state: "attached", timeout: 30000 });
  } catch (error) {
    throw new Error("L03 fixture did not mount: " + [...pageErrors, ...startupErrors].join("; "), { cause: error });
  }
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
      observer?.disconnect();
      clearTimeout(timer);
      resolve();
    };
    if (ready()) { resolve(); return; }
    observer = new MutationObserver(finish);
    observer.observe(node, { subtree: true, childList: true, characterData: true });
    timer = setTimeout(() => {
      observer.disconnect();
      reject(new Error("frame acknowledgement timed out at " + step));
    }, 15000);
  }), step);
  const values = {};
  for (const spec of config.specs) {
    activeCase = spec.id;
    activeStep = 0;
    try {
      const row = page.locator('[data-case="' + spec.id + '"]');
      const result = row.locator("[data-result]");
      await result.waitFor({ state: "attached", timeout: 30000 });
      await waitForFrame(result, 0);
      for (let step = 1; step < spec.frames; step++) {
        activeStep = step;
        await row.locator("[data-next]").click();
        await waitForFrame(result, step);
      }
      const observed = JSON.parse(await result.textContent());
      observed.console = diagnostics.filter(d => d.case === spec.id);
      values[spec.id] = "DOM|" + JSON.stringify(canonical(observed));
    } catch (error) {
      values[spec.id] = "DOM_ERROR|" + JSON.stringify({ step: activeStep, message: error.message });
    }
  }
  process.stdout.write(JSON.stringify(values));
} catch (error) {
  process.stderr.write(error.stack + "\n");
  process.exitCode = 1;
} finally {
  await browser?.close();
}
