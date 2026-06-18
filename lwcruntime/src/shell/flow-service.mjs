export function readFlowContext() {
  const node = document.getElementById("glade-lwc-context");
  if (!node) {
    return {};
  }
  try {
    return JSON.parse(node.textContent || "{}").flow || {};
  } catch (_err) {
    return {};
  }
}

export function dispatchStatusChange(element, detail = {}) {
  element?.dispatchEvent?.(new CustomEvent("statuschange", { detail, bubbles: true, composed: true }));
}

export function captureFlowEvent(event) {
  const detail = {
    type: event.type,
    detail: event.detail || {},
  };
  document.dispatchEvent(new CustomEvent("glade:flow-event", { detail }));
  return detail;
}
