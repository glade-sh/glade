// Local replay of the owned L18 native fixture. No native answers are supplied.
import fs from "node:fs";
import { launchBrowser, runRows } from "./browser_row_pool.mjs";
const config = JSON.parse(fs.readFileSync(0, "utf8"));
const browser = await launchBrowser(config);

// Same field/message projection as the native capture, copied unchanged.
const observeDOM = node=>{
 const seen=new Set(),fields=[],messages=[];
 function walk(root){
  if(!root||seen.has(root))return;seen.add(root);
  for(const element of root.querySelectorAll('*')){
   const visible=element.getClientRects().length>0 && getComputedStyle(element).visibility!=='hidden';
   if(visible && ['INPUT','SELECT','TEXTAREA'].includes(element.tagName))fields.push({tag:element.tagName,type:element.type,value:element.value,checked:element.checked,disabled:element.disabled,required:element.required,readOnly:element.readOnly,ariaInvalid:element.getAttribute('aria-invalid'),ariaExpanded:element.getAttribute('aria-expanded'),validationMessage:element.validationMessage});
   if(visible && element.matches('.slds-form-element__help,[role="alert"]'))messages.push(element.textContent);
   if(element.shadowRoot)walk(element.shadowRoot);
  }
 }
 walk(node);if(node.shadowRoot)walk(node.shadowRoot);
 return {fields,messages,shadowAccessible:!!node.shadowRoot,observationWindowMs:500,fieldObservation:fields.length?'rendered fields observed':'no accessible rendered field within observation window'};
};

// Native TSV text uses sorted keys, compact JSON and ASCII escapes. Serialize
// in JavaScript to retain omitted properties, raw null and lone surrogates.
function nativeText(value) {
  function sorted(node) {
    if (Array.isArray(node)) return node.map(sorted);
    if (node !== null && typeof node === "object") {
      return Object.fromEntries(Object.keys(node).sort().map(key => [key, sorted(node[key])]));
    }
    return node;
  }
  return "BROWSER|" + JSON.stringify(sorted(value)).replace(/[\u007f-\uffff]/g,
    unit => "\\u" + unit.charCodeAt(0).toString(16).padStart(4, "0"));
}

// Each worker context owns its page, listeners and row state. Every row
// navigates a fresh page URL, so rows are independent of their order.
async function rowWorker(context) {
  const page = await context.newPage();
  const markerSelector = "[data-l18-host]:visible [data-result]";
  const fixtureSelector = "[data-l18-host]:visible [data-fixture]";
  let pageErrors = [], assetErrors = [];
  page.on("pageerror", error => pageErrors.push(error.message));
  page.on("response", response => {
    if (response.status() >= 400 && /\.(?:m?js|css)$/.test(new URL(response.url()).pathname)) {
      assetErrors.push(response.status() + " " + new URL(response.url()).pathname);
    }
  });
  async function state(id, stages) {
    const deadline = Date.now() + 15000;
    while (Date.now() < deadline) {
      if (assetErrors.length) throw new Error("L18 local asset unavailable: " + assetErrors.join("; "));
      const marker = page.locator(markerSelector);
      const count = await marker.count();
      if (count > 1) throw new Error("L18 ambiguous row marker");
      if (count === 1) {
        let value;
        try { value = JSON.parse(await marker.textContent({ timeout: 1000 })); }
        catch { /* The fixture publishes its acknowledgement asynchronously. */ }
        if (value?.id === id && (stages.includes(value.stage) ||
            (value.stage === "done" && value.observation?.nativeRenderError))) return value;
      }
      if (pageErrors.length) throw new Error("L18 local page error: " + pageErrors.join("; "));
      await page.waitForTimeout(25);
    }
    throw new Error("L18 owned acknowledgement timeout for " + id);
  }
  async function inputAction(spec, selector, action, value, label) {
    const fixture = page.locator(fixtureSelector), marker = page.locator(markerSelector);
    async function owned() {
      if (await fixture.count() !== 1 || await marker.count() !== 1) return false;
      const current = JSON.parse(await marker.textContent({ timeout: 1000 }));
      return current.id === spec.id && current.stage === "interaction-ready";
    }
    if (!await owned()) throw new Error("L18 interaction row ownership mismatch");
    const node = action === "option" ? fixture.getByText(label, { exact: true }) : page.locator(selector);
    await node.waitFor({ state: "attached", timeout: 5000 });
    if (await node.count() !== 1) throw new Error("L18 interaction target mismatch");
    const target = await node.evaluate(n => ({ tag: n.tagName, type: n.type || null,
      value: n.value === undefined ? null : n.value, disabled: !!n.disabled,
      visible: !!n.getClientRects().length && getComputedStyle(n).visibility !== "hidden" }));
    const windowMs = 5000;
    try {
      if (action === "fill") {
        await node.fill(value, { timeout: windowMs });
        await node.press("Tab", { timeout: windowMs });
      } else if (action === "select") {
        await node.selectOption(value, { timeout: windowMs });
        await node.press("Tab", { timeout: windowMs });
      } else if (action === "click" || action === "option") {
        await node.click({ timeout: windowMs });
      } else throw new Error("L18 unsupported input action");
      return { status: "acted" };
    } catch (error) {
      if (error.name !== "TimeoutError" || !await owned()) throw error;
      return { status: "bounded-unavailable", kind: "native-target-not-actionable",
        action, windowMs: ["fill", "select"].includes(action) ? 2 * windowMs : windowMs,
        stepTimeoutMs: windowMs, target, reason: "NATIVE_ACTIONABILITY_TIMEOUT" };
    }
  }
  return async spec => {
    let stage = "navigate-row";
    try {
      pageErrors = []; assetErrors = [];
      const url = new URL(config.url);
      url.searchParams.set("c__case", spec.id);
      const response = await page.goto(url.href, { waitUntil: "domcontentloaded", timeout: 30000 });
      if (!response?.ok()) throw new Error("L18 local page unavailable");
      stage = "row-ready";
      let value = await state(spec.id, ["ready"]);
      if (value.stage === "done") return { value: nativeText(value) };
      stage = "row-execute";
      await page.locator("[data-l18-host]:visible [data-run]").click({ timeout: 15000 });
      stage = "row-acknowledgement";
      value = await state(spec.id, ["done", "interaction-ready"]);
      const bounded = [];
      if (value.stage === "interaction-ready") {
        stage = "native-interaction";
        async function act(selector, action, value, label) {
          const observation = await inputAction(spec, selector, action, value, label);
          if (observation.status === "bounded-unavailable") { bounded.push(observation); return false; }
          return true;
        }
        if (spec.interaction === "type") await act(fixtureSelector + " input", "fill", spec.typed);
        else if (spec.interaction === "select") {
          await act(spec.component === "select" ? fixtureSelector : fixtureSelector + " select", "select", spec.typed);
        } else if (["open", "choose"].includes(spec.interaction)) {
          const opened = await act(fixtureSelector + ' [role="combobox"]', "click");
          if (spec.interaction === "choose" && opened) {
            await act(fixtureSelector + ' [role="option"][data-value="' + spec.typed + '"]', "click");
          }
        } else if (["radio", "checkbox"].includes(spec.interaction)) {
          const label = ({ "": "Empty", a: "Alpha", b: "Beta" })[spec.typed] ?? "Owned input";
          await act(fixtureSelector, "option", undefined, label);
        } else throw new Error("L18 unknown input interaction");
        await page.locator("[data-l18-host]:visible [data-finish]").click({ timeout: 15000 });
        stage = "row-done";
        value = await state(spec.id, ["done"]);
      }
      if (value.id !== spec.id || value.stage !== "done" || !value.observation) {
        throw new Error("L18 row acknowledgement invalid");
      }
      if (bounded.length) value.observation.boundedInteraction = bounded;
      if (value.observation.kind !== "bounded-no-component" && !value.observation.nativeRenderError) {
        value.observation.dom = await page.locator(fixtureSelector).evaluate(observeDOM);
      }
      return { value: nativeText(value) };
    } catch (error) {
      if (assetErrors.length || page.isClosed()) throw error;
      return { error: stage + ": " + error.message };
    }
  };
}

try {
  const rows = await runRows(browser, config.cases, { setup: rowWorker, run: (observe, spec) => observe(spec) });
  const values = {}, errors = {};
  config.cases.forEach((spec, index) => {
    if ("value" in rows[index]) values[spec.id] = rows[index].value;
    else errors[spec.id] = rows[index].error;
  });
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
