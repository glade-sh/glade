import { diagnostics } from "./diagnostics.mjs";

const CONTEXT_PANEL_STORAGE_KEY = "glade:context-panel:v1";
const flowEvents = window.__gladeFlowEvents || [];
window.__gladeFlowEvents = flowEvents;

export function renderContextPanel(root, config = {}) {
  if (!root) {
    return;
  }
  const pageRef = config.pageReference || {};
  const attrs = pageRef.attributes || {};
  const context = readShellContext();
  const rows = [
    ["Type", pageRef.type || context.kind || "local"],
    ["Target", context.kind],
    ["Component", attrs.componentName || context.componentName],
    ["Object", attrs.objectApiName || context.objectApiName],
    ["Record", attrs.recordId || context.recordId],
    ["Form factor", context.formFactor || attrs.formFactor],
    ["Page", context.pageName],
    ["App", context.appName],
    ["Tab", context.tabName],
    ["Flow", context.flow?.apiName],
    ["Community", context.community?.site],
  ].filter((row) => row[0] === "Type" || String(row[1] || "").trim() !== "");
  root.classList.add("glade-context-panel");
  const collapsed = readContextPanelCollapsed();
  root.innerHTML = `
    <header class="glade-context-header">
      <h2>Context</h2>
      <button class="glade-context-toggle" type="button" data-glade-context-toggle aria-label="Collapse context" title="Collapse context">→</button>
    </header>
    <div data-glade-context-body>
      <dl>
        ${rows.map(([label, value]) => `<dt>${escapeHTML(label)}</dt><dd>${escapeHTML(value || "")}</dd>`).join("")}
      </dl>
      <h2>Diagnostics</h2>
      <ul data-glade-diagnostics></ul>
      <h2>Flow Events</h2>
      <ul data-glade-flow-events></ul>
    </div>
  `;
  root.querySelector("[data-glade-context-toggle]")?.addEventListener("click", () => {
    const next = root.dataset.gladeContextCollapsed !== "true";
    persistContextPanelCollapsed(next);
    applyContextPanelState(root, next);
  });
  applyContextPanelState(root, collapsed);
  renderDiagnostics(root.querySelector("[data-glade-diagnostics]"));
  renderFlowEvents(root);
}

function applyContextPanelState(root, collapsed) {
  root.dataset.gladeContextCollapsed = String(collapsed);
  const workbench = root.closest("[data-glade-workbench-console]");
  if (workbench) {
    workbench.dataset.gladeContextCollapsed = String(collapsed);
  }
  const body = root.querySelector("[data-glade-context-body]");
  if (body) {
    body.hidden = collapsed;
  }
  const toggle = root.querySelector("[data-glade-context-toggle]");
  if (toggle) {
    const label = collapsed ? "Expand context" : "Collapse context";
    toggle.textContent = collapsed ? "←" : "→";
    toggle.setAttribute("aria-label", label);
    toggle.setAttribute("title", label);
    toggle.setAttribute("aria-expanded", String(!collapsed));
  }
}

function readContextPanelCollapsed() {
  if (typeof sessionStorage === "undefined") {
    return false;
  }
  try {
    const parsed = JSON.parse(sessionStorage.getItem(CONTEXT_PANEL_STORAGE_KEY) || "{}");
    return Boolean(parsed.collapsed);
  } catch (_err) {
    return false;
  }
}

function persistContextPanelCollapsed(collapsed) {
  if (typeof sessionStorage === "undefined") {
    return;
  }
  try {
    sessionStorage.setItem(CONTEXT_PANEL_STORAGE_KEY, JSON.stringify({ collapsed: Boolean(collapsed) }));
  } catch (_err) {
    // Context panel persistence should never block preview work.
  }
}

function readShellContext() {
  const node = document.getElementById("glade-lwc-context");
  if (!node) {
    return {};
  }
  try {
    return JSON.parse(node.textContent || "{}");
  } catch (_err) {
    return {};
  }
}

export function renderDiagnostics(list) {
  if (!list) {
    return;
  }
  list.replaceChildren(...diagnostics.map((diag) => {
    const item = document.createElement("li");
    item.textContent = `${diag.code}: ${diag.message}`;
    return item;
  }));
}

export function recordFlowEvent(event) {
  const payload = event?.detail && typeof event.detail === "object" ? event.detail : {};
  flowEvents.push({
    type: payload.type || event?.type || "",
    detail: payload.detail || {},
  });
  renderFlowEvents(document);
}

export function getFlowEvents() {
  return [...flowEvents];
}

export function clearFlowEvents() {
  flowEvents.splice(0, flowEvents.length);
  renderFlowEvents(document);
}

export function renderFlowEvents(root = document) {
  for (const list of root.querySelectorAll("[data-glade-flow-events]")) {
    list.replaceChildren(...flowEvents.map((event) => {
      const item = document.createElement("li");
      item.textContent = `${event.type}: ${JSON.stringify(event.detail || {})}`;
      return item;
    }));
  }
}

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  })[ch]);
}
