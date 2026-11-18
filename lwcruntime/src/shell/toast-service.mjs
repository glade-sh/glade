const toasts = window.__gladeToasts || [];
window.__gladeToasts = toasts;

let nextToastId = 1;

// Auto-dismiss durations, in milliseconds, measured from the push. These are
// MEASUREMENTS, not documented Salesforce constants. The API 67.0 and 68.0
// captures (release 68 org, polled every 100 ms for 30 s) record, per run, the
// last poll that still saw the toast (lastSeenMs) and the first poll that saw it
// gone (removalDetectedMs), both relative to the push. Each value here is the
// midpoint of those two, averaged over the runs of that mode, rounded to 100 ms:
//   event (ShowToastEvent) dismissible: seen to 20688-20786, gone by 20799-20891
//   event pester: seen to 20771-20804, gone by 20875-20907
//   module (lightning/toast) dismissible: seen to 5197-5299, gone by 5302-5404
//   event sticky, module pester, module sticky: still shown at 30 s
// The poll interval leaves the true timer anywhere inside each interval, about
// +/-0.1 s. The export is frozen; setToastClock is the only test seam.
export const TOAST_DURATIONS_MS = Object.freeze({
  event: Object.freeze({ dismissible: 20800, pester: 20800 }),
  module: Object.freeze({ dismissible: 5300 }),
});

// ShowToastEvent keeps its public detail to the captured { label } envelope,
// so the options that the event cannot carry stay beside the event object.
const eventOptions = new WeakMap();
export function rememberToastEvent(event, options) {
  eventOptions.set(event, { ...options });
}

let clock = {
  setTimeout: (fn, ms) => window.setTimeout(fn, ms),
  clearTimeout: (id) => window.clearTimeout(id),
};
// Tests replace the clock; pass nothing to restore the window timers.
export function setToastClock(next) {
  clock = next || {
    setTimeout: (fn, ms) => window.setTimeout(fn, ms),
    clearTimeout: (id) => window.clearTimeout(id),
  };
}

export function installToastService(root = document.body) {
  ensureToastRegion(root);
  const onToast = (event) => {
    const detail = event.detail || {};
    recordToast(eventOptions.has(event) ? { ...eventOptions.get(event), ...detail, title: eventOptions.get(event).title ?? detail.label } : detail);
  };
  document.addEventListener("lightning__showtoast", onToast);
  renderToasts(root);
  return () => document.removeEventListener("lightning__showtoast", onToast);
}

export function recordToast(detail = {}) {
  const toast = {
    id: nextToastId++,
    title: String(detail.title || detail.label || ""),
    message: String(detail.message || ""),
    variant: String(detail.variant || "info"),
    mode: String(detail.mode || "dismissible"),
    // lightning/toast dispatches its config with a source key; a
    // ShowToastEvent does not. The two render and time out differently.
    kind: "source" in detail ? "module" : "event",
    detail: { ...detail },
  };
  toasts.push(toast);
  const duration = TOAST_DURATIONS_MS[toast.kind][toast.mode];
  if (duration !== undefined) toast.timer = clock.setTimeout(() => dismissToast(toast.id), duration);
  document.dispatchEvent(new CustomEvent("glade:toast", { detail: toast }));
  renderToasts();
  return toast;
}

export function getToasts() {
  return [...toasts];
}

export function dismissToast(id) {
  const index = toasts.findIndex((toast) => toast.id === id);
  if (index < 0) return;
  const [toast] = toasts.splice(index, 1);
  if (toast.timer !== undefined) clock.clearTimeout(toast.timer);
  renderToasts();
}

export function clearToasts() {
  for (const toast of toasts) if (toast.timer !== undefined) clock.clearTimeout(toast.timer);
  toasts.splice(0, toasts.length);
  renderToasts();
}

export function renderToasts(root = document) {
  const region = root.querySelector?.("[data-glade-toast-region]") || document.querySelector("[data-glade-toast-region]");
  if (!region) {
    return;
  }
  region.replaceChildren(...toasts.slice(-5).map(toastElement));
}

function ensureToastRegion(root) {
  if (root.querySelector?.("[data-glade-toast-region]")) {
    return;
  }
  const region = document.createElement("section");
  region.className = "glade-toast-region";
  region.dataset.gladeToastRegion = "";
  region.setAttribute("aria-live", "polite");
  region.setAttribute("aria-label", "Notifications");
  root.appendChild(region);
}

function element(tag, className, attrs = {}, children = []) {
  const el = document.createElement(tag);
  if (className) el.className = className;
  for (const [name, value] of Object.entries(attrs)) el.setAttribute(name, value);
  el.append(...children);
  return el;
}

function toastElement(toast) {
  const close = () => dismissToast(toast.id);
  if (toast.kind === "module") {
    // Captured lightning/toast: a role=alert live region around the toast, and
    // a title-only Close button in every mode.
    const button = element("button", "slds-button slds-button_icon slds-button_icon-inverse", { title: "Close", type: "button" });
    button.addEventListener("click", close);
    const content = element("div", "slds-notify__content", {}, [
      element("h2", "slds-text-heading_small", {}, [toast.title]),
      element("span", "", {}, [toast.message]),
    ]);
    const body = element("div", `slds-notify slds-notify_toast slds-theme_${toast.variant}`, { "data-f6-region": "" }, [
      content, element("div", "slds-notify__close", {}, [button]),
    ]);
    const container = element("div", `glade-toast glade-toast_${toast.variant} slds-notify_container slds-is-relative`,
      { role: "alert", "aria-live": "polite", "aria-atomic": "true", part: "toast", "data-glade-toast-id": String(toast.id) }, [body]);
    return container;
  }
  // Captured ShowToastEvent toast: no role and no live region. Dismissible and
  // sticky toasts have a Close button with an aria-label and an assistive
  // span; pester has no close button.
  const children = [element("div", "toastContent slds-notify__content slds-align-middle", {}, [
    element("strong", "", {}, [toast.title]),
    element("span", "", {}, [toast.message]),
  ])];
  if (toast.mode !== "pester") {
    const button = element("button", "slds-button slds-button_icon toastClose slds-button_icon-inverse slds-button_icon-bare", { "aria-label": "Close", type: "button" },
      [element("span", "slds-assistive-text", {}, ["Close"])]);
    button.addEventListener("click", close);
    children.push(button);
  }
  return element("div", `glade-toast glade-toast_${toast.variant} slds-theme--${toast.variant} slds-notify--toast slds-notify forceToastMessage`,
    { "data-key": toast.variant, "data-f6-region": "", "data-glade-toast-id": String(toast.id) }, children);
}
