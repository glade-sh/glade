// V12's owned native DOM observer, applied to the shared product HTTP server.
// Inputs contain fixture controls only. No Salesforce answers or credentials.
import fs from "node:fs";
import { createRequire } from "node:module";

const config = JSON.parse(fs.readFileSync(0, "utf8"));
const { chromium } = createRequire(import.meta.url)(config.playwrightModule);
const browser = await chromium.launch({
  headless: true,
  timeout: 15000,
  ...(config.executablePath ? { executablePath: config.executablePath } : {}),
});

// Keep the captured operation and observation fields unchanged. They measure
// real DOM controls, callback order and status counters from product scripts.
const snapshot = el => ({
  targets: Array.from(el.querySelectorAll("[data-target]")).map(n => ({ name: n.dataset.target, text: n.textContent })),
  inputs: Array.from(el.querySelectorAll("[data-input] input[type=text]")).map(n => ({ value: n.value, disabled: n.disabled })),
  messages: Array.from(el.querySelectorAll("[data-messages]")).map(n => n.textContent.trim()),
  callbacks: window.ownedLog.slice(),
  completed: window.ownedDone,
  starts: window.ownedStarts,
  stops: window.ownedStops,
});

// Same post-error document observation used by the native review capture.
const errorDocument = el => ({
  ownedRoots: document.querySelectorAll("[data-case]").length,
  forms: document.querySelectorAll("form").length,
  inputs: document.querySelectorAll("input").length,
  buttons: document.querySelectorAll("button").length,
  links: document.querySelectorAll("a").length,
});

const action = async (el, c) => {
  window.ownedArgument = c.value;
  const n = el.querySelector("[data-input] input[type=text]");
  if (n) n.value = c.value === null ? "" : c.value;
  const done = window.ownedDone;
  if (c.component === "support") n.dispatchEvent(new Event("change", { bubbles: true }));
  else if (c.component !== "poller") {
    const trigger = el.querySelector("[data-trigger]") || Array.from(el.querySelectorAll("input[type=submit],input[type=button],a")).find(n => (n.value || n.textContent) === "V12_RUN");
    if (!trigger) throw new Error("OWNED_TRIGGER_MISSING");
    trigger.click();
  }
  const deadline = Date.now() + (c.component === "poller" ? 8000 : 5000);
  while (Date.now() < deadline) {
    if (window.ownedDone > done) {
      await new Promise(r => setTimeout(r, 100));
      return { observation: "completed" };
    }
    await new Promise(r => setTimeout(r, 100));
  }
  return { observation: "no-completion-within-bound", boundMs: c.component === "poller" ? 8000 : 5000 };
};

// This is the native capture's visible-error line filter. Retain each selected
// line's raw text; fixture/Error ID substitutions happen after observation.
const nativeErrors = page => page.evaluate(() => {
  const pattern = /Error (?:is in|evaluating) expression|(?:System\.)?(?:QueryException|SObjectException|NullPointerException|VisualforceException|DmlException|IllegalArgumentException)|SObject row was retrieved via SOQL|(?:Invalid|No such) (?:field|column)|unexpected token|Didn.t understand relationship|Attempt to de-reference a null object|Argument cannot be null|List index out of bounds|An internal server error has occurred/i;
  const messages = Array.from(document.querySelectorAll("#errorTitle,#errorBody,.messageText,.errorMsg,.errorMessage")).map(n => n.textContent.trim());
  const lines = [...messages, ...(document.body?.innerText || "").split("\n")];
  return [...new Set(lines.map(line => line.trim()).filter(line => pattern.test(line)))].sort();
});

const bounded = async (operation, timeout, label) => {
  let timer;
  try {
    return await Promise.race([
      operation,
      new Promise((_, reject) => {
        timer = setTimeout(() => reject(new Error(label())), timeout);
      }),
    ]);
  } finally {
    clearTimeout(timer);
  }
};

const contextDestroyed = error => /Execution context was destroyed|Cannot find context with specified id/i.test(error.message);

const observeLoadedDocument = async (page, read, stage) => {
  const deadline = Date.now() + 15000;
  for (;;) {
    try {
      await page.waitForLoadState("load", { timeout: Math.max(1, deadline - Date.now()) });
      return await read();
    } catch (error) {
      if (!contextDestroyed(error) || Date.now() >= deadline) throw error;
      // A second navigation can commit between readiness and a DOM read.
      // Retry only that read; the action and its recorded outcome stay intact.
      stage("wait for navigated document");
      await new Promise(resolve => setTimeout(resolve, 100));
    }
  }
};

// Apply the native mixed-control capture's UI click and 3s target-change
// observation. Locator.click waits for full-postback navigation; the existing
// 76 DOM rows below keep their captured action/completion observer unchanged.
const observeMixed = async (page, root, spec, before, stage) => {
  const value = { before, outcomes: [] };
  for (const submission of spec.mixed_control.submissions) {
    stage(`mixed ${submission.control}`);
    await root.waitFor({ state: "attached", timeout: 5000 });
    const error = await nativeErrors(page);
    if (error.length) return { ...value, nativeError: error };
    const filled = await root.evaluate((el, value) => {
      const n = el.querySelector("[data-input] input[type=text]");
      if (!n) throw new Error("OWNED_INPUT_MISSING");
      n.value = value;
      return n.value;
    }, submission.value);
    const previousTarget = await root.locator('[data-target="target"]').textContent();
    const tags = { link: "a", function: "button", unique: spec.mixed_control.unique_command === "link" ? "a" : "input[type=submit],input[type=button]" };
    const trigger = `[data-control="${submission.control}"] :is(${tags[submission.control]})`;
    await root.locator(trigger).click({ timeout: 15000 });
    const outcome = { control: submission.control, filled };
    try {
      await page.waitForFunction(previous => {
        const target = document.querySelector('[data-target="target"]');
        return target !== null && target.textContent !== previous;
      }, previousTarget, { timeout: 3000 });
      outcome.observation = "changed-within-bound";
    } catch (error) {
      if (error.name !== "TimeoutError") throw error;
      outcome.observation = "no-change-within-bound";
      outcome.boundMs = 3000;
    }
    const afterError = await observeLoadedDocument(page, () => nativeErrors(page), stage);
    if (afterError.length) {
      outcome.observation = "native-error";
      delete outcome.boundMs;
      value.outcomes.push(outcome);
      return { ...value, nativeError: afterError };
    }
    await root.waitFor({ state: "attached", timeout: 5000 });
    outcome.after = await root.evaluate(snapshot);
    value.outcomes.push(outcome);
    value.after = outcome.after;
  }
  return value;
};

const observe = async (page, spec, stage) => {
  stage("navigation");
  await page.goto(config.url + spec.path, { waitUntil: "load", timeout: 60000 });
  stage("initial observation");
  const initialError = await nativeErrors(page);
  if (initialError.length) return { nativeError: initialError, stage: "render" };
  const root = page.locator(`[data-case="${spec.id}"]`);
  if (!(await root.count())) {
    return { localError: await page.locator("body").innerText(), stage: "render" };
  }
  const before = await root.evaluate(snapshot);
  if (spec.mixed_control) return observeMixed(page, root, spec, before, stage);
  const outcomes = [];
  for (let step = 0; step < spec.steps; step++) {
    stage(`action ${step + 1}`);
    // Install the watcher before the trigger. A form POST can navigate to the
    // same URL, and checking load after context destruction can see the old
    // document's load state before the replacement document commits.
    const navigation = page.waitForEvent("framenavigated", {
      predicate: frame => frame === page.mainFrame(),
      timeout: 60000,
    }).then(() => page.waitForLoadState("load", { timeout: 60000 }))
      .then(() => null, error => error);
    try {
      outcomes.push(await root.evaluate(action, spec));
    } catch (error) {
      // Full navigation or a missing emitted trigger is a measured local
      // difference. Preserve it in the report; never drop the native row.
      outcomes.push({ localError: error.message });
      stage("navigation after action");
      if (contextDestroyed(error)) {
        const navigationError = await navigation;
        if (navigationError) throw navigationError;
      } else {
        await page.waitForLoadState("load");
      }
      break;
    }
  }
  stage("final observation");
  return observeLoadedDocument(page, async () => {
    const error = await nativeErrors(page);
    if (error.length) return { before, outcomes, nativeError: error };
    if (await root.count()) return { before, outcomes, after: await root.evaluate(snapshot) };
    return { before, outcomes, localError: await page.locator("body").innerText(), stage: "observe" };
  }, stage);
};

try {
  for (const spec of config.specs) {
    let context;
    let currentStage = "create context";
    const stage = value => {
      currentStage = value;
      process.stderr.write(`V12 ${spec.id}: ${value}\n`);
    };
    let value;
    try {
      value = await bounded((async () => {
        stage("create context");
        context = await browser.newContext();
        // A fresh context per row releases browser state and serves only local
        // requests. The captured action bounds above remain 5s and 8s.
        await context.route("**/*", route => new URL(route.request().url()).origin === config.url ? route.continue() : route.abort());
        const page = await context.newPage();
        const value = await observe(page, spec, stage);
        if (spec.error_document) {
          value.errorDocument = await page.locator("body").evaluate(errorDocument);
        }
        return value;
      })(), 90000, () => `V12 ${spec.id}: browser stalled at ${currentStage} (90s)`);
    } finally {
      if (context) {
        stage("close context");
        await bounded(context.close(), 15000, () => `V12 ${spec.id}: context cleanup stalled (15s)`);
      }
    }
    process.stdout.write(JSON.stringify({ id: spec.id, value }) + "\n");
  }
} finally {
  await bounded(browser.close(), 15000, () => "V12 browser cleanup stalled (15s)");
}
