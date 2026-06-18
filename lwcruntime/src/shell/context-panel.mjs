import { diagnostics } from "./diagnostics.mjs";

const flowEvents = window.__gladeFlowEvents || [];
window.__gladeFlowEvents = flowEvents;

export function renderContextPanel(root, config = {}) {
  if (!root) {
    return;
  }
  const pageRef = config.pageReference || {};
  const attrs = pageRef.attributes || {};
  root.classList.add("glade-context-panel");
  root.innerHTML = `
    <h2>Context</h2>
    <dl>
      <dt>Type</dt><dd>${escapeHTML(pageRef.type || "local")}</dd>
      <dt>Object</dt><dd>${escapeHTML(attrs.objectApiName || "")}</dd>
      <dt>Record</dt><dd>${escapeHTML(attrs.recordId || "")}</dd>
    </dl>
    <h2>Diagnostics</h2>
    <ul data-glade-diagnostics></ul>
    <h2>Flow Events</h2>
    <ul data-glade-flow-events></ul>
  `;
  renderDiagnostics(root.querySelector("[data-glade-diagnostics]"));
  renderFlowEvents(root);
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
