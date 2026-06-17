import { diagnostics } from "./diagnostics.mjs";

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
  `;
  renderDiagnostics(root.querySelector("[data-glade-diagnostics]"));
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

function escapeHTML(value) {
  return String(value ?? "").replace(/[&<>"']/g, (ch) => ({
    "&": "&amp;",
    "<": "&lt;",
    ">": "&gt;",
    '"': "&quot;",
    "'": "&#39;",
  })[ch]);
}
