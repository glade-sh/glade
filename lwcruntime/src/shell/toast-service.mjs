const toasts = window.__gladeToasts || [];
window.__gladeToasts = toasts;

let nextToastId = 1;

export function installToastService(root = document.body) {
  ensureToastRegion(root);
  const onToast = (event) => {
    recordToast(event.detail || {});
  };
  document.addEventListener("lightning__showtoast", onToast);
  renderToasts(root);
  return () => document.removeEventListener("lightning__showtoast", onToast);
}

export function recordToast(detail = {}) {
  const toast = {
    id: nextToastId++,
    title: String(detail.title || ""),
    message: String(detail.message || ""),
    variant: String(detail.variant || "info"),
    mode: String(detail.mode || "dismissible"),
    detail: { ...detail },
  };
  toasts.push(toast);
  document.dispatchEvent(new CustomEvent("glade:toast", { detail: toast }));
  renderToasts();
  return toast;
}

export function getToasts() {
  return [...toasts];
}

export function clearToasts() {
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

function toastElement(toast) {
  const el = document.createElement("div");
  el.className = `glade-toast glade-toast_${toast.variant}`;
  el.dataset.gladeToastId = String(toast.id);
  el.setAttribute("role", toast.variant === "error" ? "alert" : "status");

  const title = document.createElement("strong");
  title.textContent = toast.title;
  el.appendChild(title);

  const message = document.createElement("span");
  message.textContent = toast.message;
  el.appendChild(message);
  return el;
}
