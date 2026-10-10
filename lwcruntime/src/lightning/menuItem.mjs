import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";

function renderMenuItem($api, $cmp) {
  const { h, t, b } = $api;
  return [h("div", { key: 0, classMap: { "slds-dropdown__item": true } }, [
    h("a", {
      key: 1,
      attrs: { href: $cmp.href, role: "menuitem", tabindex: "-1" },
      on: { click: b($cmp.handleSelect) },
    }, [h("span", { key: 2 }, [t($cmp.label == null ? "" : String($cmp.label))])]),
  ])];
}
renderMenuItem.stylesheets = [];
const template = registerTemplate(renderMenuItem);
freezeTemplate(renderMenuItem);

class MenuItem extends LightningElement {
  constructor() {
    super();
    this.href = "javascript:void(0)";
  }

  connectedCallback() {
    this.setAttribute("role", "presentation");
  }

  handleSelect(event) {
    if (this.href === "javascript:void(0)" || this.disabled) event.preventDefault();
    if (this.disabled) return;
    this.dispatchEvent(new CustomEvent("privatemenuselect", {
      bubbles: true,
      composed: true,
      detail: { value: this.value },
    }));
  }
}

registerDecorators(MenuItem, {
  publicProps: {
    disabled: { config: 0 },
    href: { config: 0 },
    label: { config: 0 },
    value: { config: 0 },
  },
});
export default registerComponent(MenuItem, {
  tmpl: template,
  sel: "lightning-menu-item",
  apiVersion: 63,
});
