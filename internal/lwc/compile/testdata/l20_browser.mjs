// Local replay of the owned L20 capture controls and DOM projections.
// Configuration contains input specs only; native answers never enter this driver.
import fs from "node:fs";
import { launchBrowser, runRows } from "./browser_row_pool.mjs";

const config = JSON.parse(fs.readFileSync(0, "utf8"));
const browser = await launchBrowser(config);
const host = "[data-l20-host]:visible";
const control = host + " [data-control]:visible";
const resultSelector = host + " [data-result]";
const pause = ms => new Promise(resolve => setTimeout(resolve, ms));
const first = (selector, index = 0) => `:nth-match(${selector}, ${index + 1})`;

// These fields and selectors are the native oracle's projection, including
// DOM order, visibility, event flags, API results and explicit raw null values.
const projectNodes = nodes => nodes.map(el => ({
  control: el.hasAttribute("data-control"), tag: el.tagName.toLowerCase(), role: el.getAttribute("role"),
  text: (el.innerText || el.textContent || "").replace(/\s+/g, " ").trim(),
  expanded: el.getAttribute("aria-expanded"), selected: el.getAttribute("aria-selected"), help: el.classList.contains("slds-form-element__help"),
  type: el.tagName === "INPUT" ? el.type : null, value: el.tagName === "INPUT" ? el.value : null,
  checked: el.tagName === "INPUT" ? el.checked : null, disabled: !!el.disabled,
  label: el.getAttribute("aria-label"), title: el.title || "", href: el.tagName === "A" ? el.getAttribute("href") : null,
}));

// Each worker context owns its page. Every row navigates a fresh page URL and
// reads only that document's state, so rows are independent of their order.
async function rowWorker(context) {
  const page = await context.newPage();
  const nodes = selector => page.locator(selector).evaluateAll(projectNodes);
  const readResult = async () => JSON.parse(await page.locator(resultSelector).textContent());
  const waitStage = async (id, stages) => {
    const deadline = Date.now() + 10000;
    while (Date.now() < deadline) {
      const loader = await page.evaluate(() => window.__l20Error);
      if (loader) throw new Error("LOCAL_MODULE_ERROR: " + JSON.stringify(loader));
      if (await page.locator(resultSelector).count()) {
        const value = await readResult();
        if (value.id === id && stages.includes(value.stage)) return value;
      }
      await pause(100);
    }
    throw new Error("LOCAL_ACK_TIMEOUT: " + stages.join("/"));
  };
  const snapshot = async () => {
    const selectors = [control, ...[
      "tbody tr", '[role="treeitem"]', "td", '[role="option"]', "input",
      '[role="alert"]', '[role="status"]', ".slds-form-element__help", "a", "button",
    ].map(part => control + " " + part + ":visible")];
    const rendered = await nodes(selectors.join(","));
    return {
      controlPresent: rendered.some(n => n.control),
      rows: rendered.filter(n => n.tag === "tr" || n.role === "treeitem").map(n => ({ text: n.text, expanded: n.expanded, selected: n.selected })),
      cells: rendered.filter(n => n.tag === "td").map(n => n.text),
      options: rendered.filter(n => n.role === "option").map(n => ({ text: n.text, selected: n.selected })),
      inputs: rendered.filter(n => n.tag === "input").map(n => ({ type: n.type, value: ["checkbox", "radio"].includes(n.type) ? null : n.value, checked: n.checked, disabled: n.disabled })),
      alerts: rendered.filter(n => ["alert", "status"].includes(n.role) || n.help).map(n => n.text),
      links: rendered.filter(n => n.tag === "a").map(n => ({ text: n.text, href: n.href })),
      buttons: rendered.filter(n => n.tag === "button").map(n => ({ text: n.text, label: n.label, title: n.title, disabled: n.disabled })),
    };
  };

  const drive = async spec => {
    const actions = [], operation = spec.operation;
    const act = async (label, selector, index = 0, optional = false) => {
      const chosen = first(selector, index), deadline = Date.now() + 2000;
      let matches;
      while (true) {
        matches = await nodes(chosen);
        if (matches.length) break;
        if (Date.now() >= deadline) {
          if (!optional && ["normal", "conversion"].includes(spec.category)) {
            throw new Error("ROW_CONTROL_UNAVAILABLE: " + label);
          }
          actions.push({ operation: label, noControlWithinMs: 2000 });
          return false;
        }
        await pause(100);
      }
      const blocked = chosen + ':is([aria-disabled="true"],[aria-disabled="true"] *)';
      if (matches[0].disabled || (await nodes(blocked)).length) {
        actions.push({ operation: label, blockedByDisabledControl: true });
        return false;
      }
      await page.locator(chosen).click({ timeout: 3000 });
      actions.push({ operation: label, clicked: true });
      await pause(350);
      return true;
    };
    await pause(750);
    let before = await snapshot();
    if ((spec.data?.length || spec.items?.length) && ["normal", "conversion"].includes(spec.category)) {
      const deadline = Date.now() + 4000;
      while (!(before.rows.length || before.cells.length)) {
        if (Date.now() >= deadline) throw new Error("ROW_RENDERED_DATA_UNAVAILABLE");
        await pause(100);
        before = await snapshot();
      }
    }
    if (operation === "sort") {
      for (let i = 0; i < spec.clicks; i++) {
        await act("sort-header", control + ' th :is(a,button,[role="button"]):has-text("Value"):visible');
      }
    } else if (operation === "select" && ["datatable", "tree-grid"].includes(spec.group)) {
      for (const index of spec.selectIndexes || [0]) {
        const row = first(control + " tbody tr:visible", index);
        const selector = first(row + ' input:is([type="checkbox"],[type="radio"])');
        const deadline = Date.now() + 2000;
        let inputs;
        while (true) {
          inputs = await page.locator(selector).evaluateAll(nodes => nodes.map(input => ({
            id: input.id, type: input.type, checked: input.checked, disabled: input.disabled,
          })));
          if (inputs.length) break;
          if (Date.now() >= deadline) {
            if (["normal", "conversion"].includes(spec.category)) throw new Error("ROW_SELECTION_INPUT_UNAVAILABLE");
            actions.push({ operation: "select-row-" + index, noControlWithinMs: 2000 });
            break;
          }
          await pause(100);
        }
        if (!inputs.length) continue;
        const state = inputs[0];
        if (state.disabled) {
          actions.push({ operation: "select-row-" + index, blockedByDisabledControl: true });
          continue;
        }
        if (!["checkbox", "radio"].includes(state.type) || !state.id) throw new Error("ROW_SELECTION_INPUT_INVALID");
        await act("select-row-" + index, row + " label[for=" + JSON.stringify(state.id) + "]:visible");
      }
    } else if (operation === "action") {
      if (await act("row-actions", control + ' tbody tr :is(button[title*="action" i],button[aria-label*="action" i],button:has-text("Actions")):visible')) {
        await act("row-action-item", control + ' [role="menuitem"]:visible');
      }
    } else if (operation === "edit") {
      if (await act("open-inline-edit", host + " [data-edit]")) {
        const selector = first(control + ' input:not([type="checkbox"]):not([type="radio"]):not([type="hidden"]):visible');
        if (!(await nodes(selector)).length) throw new Error("ROW_EDITOR_UNAVAILABLE");
        // These are input events, never synthetic component outcome events.
        await page.locator(selector).evaluate((input, value) => {
          input.focus(); input.value = value;
          input.dispatchEvent(new Event("input", { bubbles: true, composed: true }));
          input.dispatchEvent(new Event("change", { bubbles: true, composed: true }));
          input.dispatchEvent(new KeyboardEvent("keydown", { key: "Enter", code: "Enter", bubbles: true, composed: true }));
          input.dispatchEvent(new KeyboardEvent("keyup", { key: "Enter", code: "Enter", bubbles: true, composed: true }));
          input.blur();
        }, spec.editValue);
        actions.push({ operation: "edit-input", value: spec.editValue });
        await pause(500);
        const apply = control + ' button:is(:text-is("Apply"),:text-is("Done")):visible';
        if ((await nodes(apply)).length) await act("editor-apply", apply);
        await act("save-draft", control + ' button:text-is("Save"):visible');
      }
    } else if (spec.group === "tree" && operation === "select") {
      await act("tree-select", control + ' [role="treeitem"] :is(a,.slds-tree__item-label):visible');
    } else if (["expand", "collapse"].includes(operation)) {
      await act(operation, control + ' :is([role="treeitem"],tbody tr) button:is([title*="expand" i],[title*="collapse" i],[aria-label*="expand" i],[aria-label*="collapse" i],:has-text("Expand"),:has-text("Collapse")):visible');
    } else if (["expandAll", "collapseAll", "validate"].includes(operation)) {
      await act(operation, host + " [data-method]");
    } else if (spec.group === "dual-listbox" && ["move", "remove", "up", "down"].includes(operation)) {
      const box = first(control + ' [role="listbox"]', operation === "move" ? 0 : 1);
      if (await act("choose-option", box + ' [role="option"]:visible', operation === "up" ? 1 : 0)) {
        const label = { move: "Move selection to Selected", remove: "Move selection to Available", up: "Move selection up", down: "Move selection down" }[operation];
        await act(operation, control + " button:has-text(" + JSON.stringify(label) + "):visible");
      }
    } else if (spec.group === "lookup-style-lists" && operation === "select") {
      if (await act("open-list", control + ' [role="combobox"]:visible')) {
        await act("select-option", control + ' [role="option"]:visible');
      }
    }
    await pause(750);
    const after = await snapshot();
    const observation = { before, after, actions, settleMs: 750 };
    if (after.controlPresent && !(after.rows.length || after.cells.length || after.options.length)) {
      observation.noVisibleRowsOrOptionsWithinMs = 750;
    }
    return observation;
  };

  return async spec => {
    let stage = "navigate-row";
    try {
      await page.goto(config.url + "?c__case=" + encodeURIComponent(spec.id), { waitUntil: "domcontentloaded", timeout: 30000 });
      stage = "row-ready";
      await waitStage(spec.id, ["ready"]);
      stage = "mount";
      await page.locator(host + " [data-mount]").click({ timeout: 5000 });
      const mounted = await waitStage(spec.id, ["mounted", "failed"]);
      if (mounted.stage === "failed") {
        return { value: { ...mounted, stage: "done", operation: "mount" } };
      }
      stage = "row-operation";
      const dom = await drive(spec);
      stage = "row-finish";
      await page.locator(host + " [data-finish]").click({ timeout: 5000 });
      const done = await waitStage(spec.id, ["done", "failed"]);
      if (spec.operation === "sort" && !done.events?.some(e => e.type === "sort")) {
        throw new Error("ROW_SORT_EVENT_UNAVAILABLE");
      }
      if (["normal", "conversion"].includes(spec.category)) {
        let expected = { action: "rowaction", edit: "save", expandAll: "method", collapseAll: "method", validate: "method", move: "change", remove: "change", up: "change", down: "change" }[spec.operation];
        if (spec.operation === "select") expected = { datatable: "rowselection", "tree-grid": "rowselection", tree: "select", "lookup-style-lists": "change" }[spec.group];
        if (spec.group === "tree-grid" && ["expand", "collapse"].includes(spec.operation)) expected = "toggle";
        if (expected && !done.events?.some(e => e.type === expected)) throw new Error("ROW_OPERATION_EVENT_UNAVAILABLE: " + expected);
        if (spec.group === "tree" && ["expand", "collapse"].includes(spec.operation)) {
          if (JSON.stringify(dom.before.rows.map(r => r.expanded)) === JSON.stringify(dom.after.rows.map(r => r.expanded))) {
            throw new Error("ROW_EXPANSION_STATE_UNCHANGED");
          }
        }
      }
      return { value: { ...done, stage: "done", dom } };
    } catch (error) {
      return { error: stage + ": " + error.message };
    }
  };
}

try {
  const rows = await runRows(browser, config.specs, { setup: rowWorker, run: (observe, spec) => observe(spec) });
  const values = {}, errors = {};
  config.specs.forEach((spec, index) => {
    if ("value" in rows[index]) values[spec.id] = rows[index].value;
    else errors[spec.id] = rows[index].error;
  });
  process.stdout.write(JSON.stringify({ values, errors }));
} finally {
  await browser.close();
}
