import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";

function renderTab($api, $cmp, $slotset) {
  return $cmp.hasBeenActive ? [$api.s("", { key: 0 }, [], $slotset)] : [];
}
renderTab.stylesheets = [];
renderTab.slots = [""];
const template = registerTemplate(renderTab);
freezeTemplate(renderTab);

class Tab extends LightningElement {
  constructor() {
    super();
    this.hasBeenActive = false;
    this.registration = {
      getLabel: () => this.label,
      getValue: () => this.value,
      setSelected: selected => this.setSelected(selected),
      activate: () => this.handleActive(),
    };
  }

  connectedCallback() {
    this.classList.add("slds-tabs_default__content", "slds-hide");
    this.setAttribute("role", "tabpanel");
    this.setAttribute("tabindex", "0");
    this.dispatchEvent(new CustomEvent("privatetabregister", {
      bubbles: true,
      composed: true,
      detail: this.registration,
    }));
  }

  get label() {
    return this.labelValue;
  }

  set label(value) {
    this.labelValue = value;
    this.registration?.update?.();
  }

  setSelected(selected) {
    this.classList.remove(selected ? "slds-hide" : "slds-show");
    this.classList.add(selected ? "slds-show" : "slds-hide");
    if (selected) this.hasBeenActive = true;
  }

  handleActive() {
    this.dispatchEvent(new CustomEvent("active"));
  }
}

registerDecorators(Tab, {
  publicProps: { label: { config: 3 }, value: { config: 0 } },
  fields: ["hasBeenActive", "labelValue"],
});
export default registerComponent(Tab, {
  tmpl: template,
  sel: "lightning-tab",
  apiVersion: 63,
});
