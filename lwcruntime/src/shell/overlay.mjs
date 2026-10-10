import {
  LightningElement, createElement, registerComponent, registerDecorators,
  registerTemplate, freezeTemplate,
} from "lwc";

// Use the same renderer as project components. Its shadow ownership keeps
// child text and fields private while exposing their composed, rendered text.
export function defineOverlayComponent(tag, render, mounted) {
  class OverlayComponent extends LightningElement {
    renderedCallback() { if (mounted) mounted(this.state, this.template); }
  }
  registerDecorators(OverlayComponent, { publicProps: { state: { config: 0 } } });
  const template = registerTemplate(render);
  freezeTemplate(render);
  return registerComponent(OverlayComponent, { tmpl: template, sel: tag, apiVersion: 63 });
}

const overlays = [];
let nextHeading = 1;

const OverlayContainer = defineOverlayComponent("lightning-overlay-container", api => [
  api.h("div", { key: 0, attrs: { "data-overlay-content": "", style: "display:contents" },
    context: { lwc: { dom: "manual" } } }, []),
], (state, root) => state.mount(root));

export function focusedControl() {
  let active = document.activeElement;
  while (active?.shadowRoot?.activeElement) active = active.shadowRoot.activeElement;
  return active;
}

export function overlayRoot() {
  const previousFocus = overlays.length ? overlays[0].previousFocus : focusedControl();
  const overlay = createElement("lightning-overlay-container", { is: OverlayContainer });
  Object.assign(overlay.style, {
    position: "fixed", inset: "0", zIndex: "2147483647", display: "flex",
    alignItems: "center", justifyContent: "center", backgroundColor: "rgba(0, 0, 0, 0.5)",
  });
  const pending = [];
  let portal;
  const state = {
    overlay, previousFocus,
    append(node) {
      if (portal) return portal.appendChild(node);
      pending.push(node);
      return node;
    },
    mount(root) {
      portal = root.querySelector("[data-overlay-content]");
      for (const node of pending) portal.appendChild(node);
      pending.length = 0;
    },
  };
  overlay.state = state;
  overlays.push(state);
  return state;
}

export function restoreOverlayFocus(state) {
  if (state.previousFocus?.isConnected && typeof state.previousFocus.focus === "function") {
    state.previousFocus.focus();
  }
}

export function removeOverlay(state) {
  state.overlay.remove();
  const index = overlays.indexOf(state);
  if (index !== -1) overlays.splice(index, 1);
}

const FormattedMessage = defineOverlayComponent("lightning-formatted-text", (api, component) => [
  api.h("p", { key: 0 }, [api.t(component.state.message)]),
]);

const PromptInput = defineOverlayComponent("lightning-input", (api, component) => {
  const state = component.state;
  return [
    api.h("div", { key: 0 }, [api.t(state.message)]),
    api.h("input", {
      key: 1, attrs: { type: "text", "aria-label": state.label,
        style: "box-sizing:border-box;width:100%;padding:0.5rem" },
      props: { value: state.value }, on: { input: state.edit },
    }, []),
  ];
});

const InteractiveDialog = defineOverlayComponent("lightning-interactive-dialog-base", (api, component) => {
  const state = component.state;
  const buttons = [];
  if (state.kind !== "alert") buttons.push(api.h("button", {
    key: 0, attrs: { type: "button", style: "padding:0.5rem 1rem;border-radius:0.25rem;cursor:pointer" },
    on: { click: state.cancel },
  }, [api.t("Cancel")]));
  buttons.push(api.h("button", {
    key: 1, attrs: { type: "button", style: "padding:0.5rem 1rem;border-radius:0.25rem;cursor:pointer" },
    on: { click: state.accept },
  }, [api.t("OK")]));
  return [api.h("section", {
    key: 0, attrs: {
      role: state.role, "aria-modal": "true", tabindex: "-1",
      "aria-label": state.headerless ? state.label : null,
      "aria-labelledby": !state.headerless && state.canFocus ? state.headingID : null,
      style: "box-sizing:border-box;width:min(28rem, calc(100vw - 2rem));padding:1.5rem;" +
        "border-radius:0.25rem;background-color:white;color:#181818;box-shadow:0 0.5rem 2rem rgba(0,0,0,0.3)",
    }, on: { keydown: state.keydown },
  }, [
    api.h("h2", { key: 0, attrs: { id: state.headingID, tabindex: "-1" },
      props: { hidden: state.headerless || state.label === "" } }, [api.t(state.headerless ? "" : state.label)]),
    api.c(state.kind === "prompt" ? "lightning-input" : "lightning-formatted-text",
      state.kind === "prompt" ? PromptInput : FormattedMessage,
      { key: 1, props: { state }, attrs: { style: "display:block" } }, []),
    api.h("div", { key: 2 }, buttons),
  ])];
});

export function openFeedback(kind, supplied = {}) {
  const options = supplied == null ? {} : supplied;
  if (typeof options !== "object") throw new Error("Invalid .open() or .open({}) argument.");
  const headerless = options.variant === "headerless";
  const defaultLabel = kind[0].toUpperCase() + kind.slice(1);
  const label = Object.prototype.hasOwnProperty.call(options, "label") ? String(options.label ?? "") : defaultLabel;
  const rawMessage = options.message;
  const message = String(rawMessage ?? "");
  const defaultValue = kind === "prompt" ? String(options.defaultValue ?? "") : "";
  // Numeric alert/confirm messages render but do not establish heading focus.
  const canFocus = kind === "prompt" || typeof rawMessage !== "number";
  const role = kind === "prompt" ? "dialog" : "alertdialog";
  return new Promise(resolve => {
    const context = overlayRoot();
    window.dispatchEvent(new CustomEvent("glade" + kind, { detail: options, bubbles: true, composed: true }));
    let dismissed = false;
    const dismiss = value => {
      if (dismissed) return;
      dismissed = true;
      restoreOverlayFocus(context);
      resolve(value);
      setTimeout(() => removeOverlay(context), 250);
    };
    const state = { kind, label, message, headerless, canFocus, role,
      headingID: "glade-dialog-heading-" + nextHeading++, value: defaultValue };
    state.edit = event => { state.value = event.target.value; };
    state.cancel = () => dismiss(kind === "prompt" ? null : false);
    state.accept = () => dismiss(kind === "prompt" ? state.value : kind === "confirm" ? true : options.result);
    const host = createElement("lightning-interactive-dialog-base", { is: InteractiveDialog });
    host.state = state;
    host.style.display = "block";
    host.setAttribute("role", role);
    state.keydown = event => {
      if (event.key === "Escape") {
        event.preventDefault();
        event.stopPropagation();
        if (kind !== "alert" || typeof rawMessage !== "number") dismiss(kind === "prompt" ? null : kind === "confirm" ? false : options.result);
      } else if (event.key === "Tab") {
        event.preventDefault();
        event.stopPropagation();
        const controls = [];
        const field = host.shadowRoot.querySelector("lightning-input");
        const input = field?.shadowRoot.querySelector("input");
        if (input) controls.push(input);
        controls.push(...host.shadowRoot.querySelectorAll("button"));
        const index = controls.indexOf(focusedControl());
        const next = event.shiftKey ? (index <= 0 ? controls.length - 1 : index - 1) :
          (index < 0 || index === controls.length - 1 ? 0 : index + 1);
        controls[next].focus();
      }
    };
    context.append(host);
    document.body.appendChild(context.overlay);
    if (canFocus) {
      const heading = host.shadowRoot.querySelector("h2");
      (heading.hidden ? host.shadowRoot.querySelector("section") : heading).focus();
    }
  });
}
