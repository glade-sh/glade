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
  api.h("div", { key: 0, attrs: { "data-modal-content": "", style: "position:relative;border-radius:0.25rem;background-color:white;color:#181818" },
    context: { lwc: { dom: "manual" } } }, []),
], (state, root) => state.mountContent(root));

// SLDS modal blueprint: section.slds-modal > div.slds-modal__container >
// close button + content. The size is a section modifier class.
const ModalChrome = defineOverlayComponent("lightning-modal-container", (api, component) => {
  const state = component.state;
  return [api.h("section", {
    key: 0, className: state.sectionClass,
    attrs: { role: "dialog", "aria-modal": "true", tabindex: "-1" },
  }, [
    api.h("div", { key: 2, classMap: { "slds-modal__container": true } }, [
      api.h("div", { key: 0, attrs: { "data-processing": "" }, props: { hidden: true } }, []),
      api.c("lightning-modal-base", ModalContent,
        { key: 1, props: { state }, attrs: { style: "display:block" } }, []),
    ]),
  ])];
});

const closeGlyph = "m310 254 130-131c6-6 6-15 0-21l-20-21c-6-6-15-6-21 0L268 212a10 10 0 0 1-14 0L123 80c-6-6-15-6-21 0l-21 21c-6 6-6 15 0 21l131 131c4 4 4 10 0 14L80 399c-6 6-6 15 0 21l21 21c6 6 15 6 21 0l131-131a10 10 0 0 1 14 0l131 131c6 6 15 6 21 0l21-21c6-6 6-15 0-21L310 268a10 10 0 0 1 0-14";

function createCloseButton() {
  const button = document.createElement("button");
  button.type = "button";
  button.className = "slds-button slds-button_icon slds-modal__close";
  button.title = "Close";
  button.setAttribute("aria-label", "Close");
  // The dismiss wrapper positions the control above the dialog.
  button.style.position = "static";
  const svgNS = "http://www.w3.org/2000/svg";
  const svg = document.createElementNS(svgNS, "svg");
  svg.setAttribute("class", "slds-button__icon slds-button__icon_large");
  svg.setAttribute("viewBox", "0 0 520 520");
  svg.setAttribute("aria-hidden", "true");
  const path = document.createElementNS(svgNS, "path");
  path.setAttribute("d", closeGlyph);
  svg.appendChild(path);
  const text = document.createElement("span");
  text.className = "slds-assistive-text";
  text.textContent = "Cancel and close";
  button.append(svg, text);
  return button;
}

export function openModal(Component, options) {
  const values = { ...options };
  const element = createElement("lightning-modal", { is: Component });
  const context = overlayRoot();
  const overlay = context.overlay;
  overlay.className = "glade-modal-overlay";
  const size = ["small", "medium", "large", "full"].includes(values.size) ? values.size : "medium";
  const sectionClass = "slds-modal slds-fade-in-open slds-modal_" + size;
  const closeButton = createCloseButton();
  const blocker = document.createElement("div");
  blocker.className = "slds-modal__container";
  blocker.dataset.container = "";
  blocker.hidden = true;
  Object.assign(blocker.style, { position: "absolute", inset: "0", zIndex: "1" });
  // Processing blocks the dismiss control. Body choices remain clickable and
  // close() itself keeps the promise pending while disableClose is truthy.
  const dismissControl = document.createElement("div");
  Object.assign(dismissControl.style, { position: "absolute", top: "-2.75rem", right: "0", display: "inline-block" });
  dismissControl.append(closeButton, blocker);
  let resolve;
  const promise = new Promise(done => { resolve = done; });
  const validLabel = values.label === undefined || typeof values.label === "string";
  const state = { context, overlay, blocker, closeButton, element, resolve, sectionClass,
    values, validLabel, disabled: false, closing: false };
  state.mountContent = root => {
    if (state.mounted) return;
    state.mounted = true;
    state.contentRoot = root.querySelector("[data-modal-content]");
    state.contentRoot.append(dismissControl);
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
  // The runtime disables the native custom element lifecycle, so LWC only
  // connects (and renders) components inserted through the DOM methods it
  // patches. Element.append() is not one of them: the modal stayed in the DOM
  // with an empty template. Use appendChild.
  state.contentRoot.appendChild(element);
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
