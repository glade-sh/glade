import { createElement } from "lwc";
import { defineOverlayComponent, overlayRoot, restoreOverlayFocus, removeOverlay } from "./overlay.mjs";

const modalStates = new WeakMap();
const activeModals = new Set();
let nextHeading = 1;

function stateFor(component) {
  return modalStates.get(component.template && component.template.host);
}

export function updateModalDisabled(component, disabled) {
  const state = stateFor(component);
  if (!state) return;
  state.disabled = disabled;
  if (state.processing) {
    state.processing.textContent = disabled ? "Processing" : "";
    state.processing.hidden = !disabled;
  }
  state.blocker.hidden = !disabled;
}

function headingIn(root) {
  if (!root) return null;
  const heading = root.querySelector("h1,h2,h3,[role=heading]");
  if (heading) return heading;
  for (const host of root.querySelectorAll("*")) {
    const heading = headingIn(host.shadowRoot);
    if (heading) return heading;
  }
  return null;
}

const ModalContent = defineOverlayComponent("lightning-modal-base", (api) => [
  api.h("div", { key: 0, attrs: { "data-modal-content": "", style: "position:relative" },
    context: { lwc: { dom: "manual" } } }, []),
], (state, root) => state.mountContent(root));

const ModalChrome = defineOverlayComponent("lightning-modal-container", (api, component) => {
  const state = component.state;
  return [api.h("section", {
    key: 0, classMap: { "slds-modal": true, "slds-fade-in-open": true },
    attrs: { role: "dialog", "aria-modal": "true", tabindex: "-1", style: state.style },
  }, [
    api.h("div", { key: 0, attrs: { "data-processing": "" }, props: { hidden: true } }, []),
    api.c("lightning-modal-base", ModalContent,
      { key: 1, props: { state }, attrs: { style: "display:block" } }, []),
  ])];
});

export function openModal(Component, options) {
  const values = { ...options };
  const element = createElement("lightning-modal", { is: Component });
  const context = overlayRoot();
  const overlay = context.overlay;
  overlay.className = "glade-modal-overlay";
  const widths = { small: "32rem", medium: "48rem", large: "64rem", full: "100vw" };
  const style = "box-sizing:border-box;width:min(" + (widths[values.size] || widths.medium) + ", 100vw);" +
    "max-height:100vh;overflow:auto;padding:1.5rem;border-radius:0.25rem;background-color:white;color:#181818";
  const closeButton = document.createElement("button");
  closeButton.type = "button";
  closeButton.className = "slds-modal__close";
  closeButton.title = "Close";
  closeButton.setAttribute("aria-label", "Close");
  closeButton.textContent = "Cancel and close";
  const blocker = document.createElement("div");
  blocker.className = "slds-modal__container";
  blocker.dataset.container = "";
  blocker.hidden = true;
  Object.assign(blocker.style, { position: "absolute", inset: "0", zIndex: "1" });
  // Processing blocks the dismiss control. Body choices remain clickable and
  // close() itself keeps the promise pending while disableClose is truthy.
  const dismissControl = document.createElement("div");
  Object.assign(dismissControl.style, { position: "relative", display: "inline-block" });
  dismissControl.append(closeButton, blocker);
  let resolve;
  const promise = new Promise(done => { resolve = done; });
  const validLabel = values.label === undefined || typeof values.label === "string";
  const state = { context, overlay, blocker, closeButton, element, resolve, style,
    values, validLabel, disabled: false, closing: false };
  state.mountContent = root => {
    if (state.mounted) return;
    state.mounted = true;
    root.querySelector("[data-modal-content]").append(dismissControl, element);
  };
  const chrome = createElement("lightning-modal-container", { is: ModalChrome });
  chrome.state = state;
  chrome.style.display = "block";
  context.append(chrome);
  modalStates.set(element, state);
  activeModals.add(state);
  for (const [key, value] of Object.entries(values)) {
    if (key.startsWith("on") && typeof value === "function") element.addEventListener(key.slice(2), value);
    else element[key] = value;
  }
  updateModalDisabled({ template: { host: element } }, element.disableClose);
  const dismiss = () => { if (!state.disabled) element.close(); };
  closeButton.addEventListener("click", dismiss);
  overlay.addEventListener("keydown", event => {
    if (event.key === "Escape") {
      event.preventDefault();
      event.stopPropagation();
      dismiss();
    }
  });
  window.dispatchEvent(new CustomEvent("lightning__modalopen", { detail: values }));
  document.body.appendChild(overlay);
  const dialog = state.dialog = chrome.shadowRoot.querySelector("section");
  state.processing = chrome.shadowRoot.querySelector("[data-processing]");
  updateModalDisabled({ template: { host: element } }, element.disableClose);
  if (validLabel) {
    let heading = headingIn(element.shadowRoot);
    if (!heading) {
      heading = document.createElement("span");
      heading.textContent = values.label || "";
      heading.hidden = true;
      chrome.shadowRoot.querySelector("lightning-modal-base").shadowRoot.querySelector("[data-modal-content]").appendChild(heading);
    }
    if (!heading.id) heading.id = "glade-modal-heading-" + nextHeading++;
    dialog.setAttribute("aria-labelledby", heading.id);
    closeButton.focus();
  }
  return promise;
}

export function closeModal(component, result) {
  const state = stateFor(component);
  if (!state || state.disabled || state.closing) return;
  state.closing = true;
  if (typeof state.values.label === "string" && state.values.label !== "") {
    state.dialog.removeAttribute("aria-labelledby");
    state.dialog.setAttribute("aria-label", state.values.label);
  }
  const pending = [...activeModals].find(other => other !== state && !other.closing);
  if (pending) pending.closeButton.focus();
  else if (!state.validLabel) state.closeButton.focus();
  else restoreOverlayFocus(state.context);
  // Repeated close() calls stay on the first closing instance until removal.
  setTimeout(() => {
    restoreOverlayFocus(state.context);
    removeOverlay(state.context);
    activeModals.delete(state);
    modalStates.delete(state.element);
    state.resolve(result);
  }, 250);
}
