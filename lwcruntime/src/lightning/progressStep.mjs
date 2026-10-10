import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import PrimitiveIcon from "./displayPrimitiveIcon.mjs";

const stateLabels = { completed: "Stage Complete", active: "Current Stage", incomplete: "Stage Not Started" };

function renderProgressStep($api, $cmp) {
  const { h, t, c } = $api;
  const completed = $cmp.state === "completed";
  const children = completed ? [c("lightning-primitive-icon", PrimitiveIcon, {
    key: 1,
    props: { iconName: "utility:check", svgClass: "slds-icon slds-icon_xx-small" },
  }, [])] : [];
  const label = $cmp.label == null ? "" : String($cmp.label);
  children.push(h("span", { key: 2, className: "slds-assistive-text" }, [
    t(`${label} - ${stateLabels[$cmp.state]}`.trim()),
  ]));
  return [h("div", {
    key: 0,
    className: "slds-progress__marker" + (completed ? " slds-progress__marker_icon slds-icon__container" : ""),
    attrs: { tabindex: "0" },
  }, children)];
}
renderProgressStep.stylesheets = [];
const template = registerTemplate(renderProgressStep);
freezeTemplate(template);

class ProgressStep extends LightningElement {
  constructor() {
    super();
    this.state = "incomplete";
    this.registration = {
      getValue: () => this.value,
      setState: state => {
        this.state = state;
        if (this.isConnected) this.updateHostClasses();
      },
    };
  }

  connectedCallback() {
    this.classList.add("slds-progress__item");
    this.setAttribute("role", "listitem");
    this.dispatchEvent(new CustomEvent("privateprogressstepregister", {
      bubbles: true,
      composed: true,
      detail: this.registration,
    }));
  }

  disconnectedCallback() {
    this.registration.unregister?.();
  }

  get value() {
    return this._value;
  }

  set value(value) {
    this._value = value;
    this.registration?.update?.();
  }

  updateHostClasses() {
    this.classList.remove("slds-is-active", "slds-is-completed");
    if (this.state === "active") this.classList.add("slds-is-active");
    if (this.state === "completed") this.classList.add("slds-is-completed");
  }
}
registerDecorators(ProgressStep, {
  publicProps: { label: { config: 0 }, value: { config: 3 } },
  fields: ["state"],
});
export default registerComponent(ProgressStep, {
  tmpl: template,
  sel: "lightning-progress-step",
  apiVersion: 63,
});
