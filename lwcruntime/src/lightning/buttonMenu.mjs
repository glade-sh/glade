import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";

const MENU_ALIGNMENTS = new Set([
  "left", "center", "right", "bottom-left", "bottom-center", "bottom-right", "auto",
]);

// Private dropdown affordance, using the bundled SLDS utility:down path.
function renderMenuIcon($api, $cmp) {
  const { h } = $api;
  return [h("svg", {
    key: 0,
    svg: true,
    className: $cmp.iconClass,
    attrs: { "aria-hidden": "true", viewBox: "0 0 520 520", focusable: "false" },
  }, [h("g", { key: 1, svg: true }, [h("path", {
    key: 2,
    svg: true,
    attrs: { d: "M83 140h354c10 0 17 13 9 22L273 374c-6 8-19 8-25 0L73 162c-7-9-1-22 10-22" },
  }, [])])])];
}
renderMenuIcon.stylesheets = [];
const iconTemplate = registerTemplate(renderMenuIcon);
freezeTemplate(renderMenuIcon);
class MenuIcon extends LightningElement {}
registerDecorators(MenuIcon, { publicProps: { iconClass: { config: 0 } } });
const ButtonMenuIcon = registerComponent(MenuIcon, {
  tmpl: iconTemplate,
  sel: "lightning-primitive-icon",
  apiVersion: 63,
});

function renderButtonMenu($api, $cmp, $slotset) {
  const { h, t, c, b, s } = $api;
  const label = $cmp.label == null ? "" : String($cmp.label);
  const buttonClass = $cmp.variant === "bare" ? "slds-button" :
    "slds-button " + (label ? "slds-button_neutral" : "slds-button_icon-border");
  const children = [];
  if (label) children.push(t(label));
  children.push(c("lightning-primitive-icon", ButtonMenuIcon, {
    key: 1,
    props: { iconClass: "slds-button__icon" + (label ? " slds-button__icon_right" : "") },
  }, []));
  children.push(h("span", { key: 2, classMap: { "slds-assistive-text": true } }, [
    t($cmp.alternativeText == null ? "" : String($cmp.alternativeText)),
  ]));
  const nodes = [h("button", {
    key: 0,
    className: buttonClass,
    attrs: {
      type: "button",
      title: $cmp.title == null ? null : String($cmp.title),
      value: $cmp.value == null ? "" : String($cmp.value),
      "aria-haspopup": "true",
      "aria-expanded": String($cmp.isOpen),
    },
    props: { disabled: Boolean($cmp.disabled) },
    on: { click: b($cmp.handleToggle) },
  }, children)];
  if ($cmp.hasRenderedMenu) {
    nodes.push(h("div", { key: 3, className: $cmp.dropdownClass }, [
      h("div", {
        key: 4,
        className: "slds-dropdown__list slds-dropdown_length-with-icon-10",
        attrs: { role: "menu" },
      }, [s("", { key: 5 }, [], $slotset)]),
    ]));
  }
  return nodes;
}
renderButtonMenu.stylesheets = [];
renderButtonMenu.slots = [""];
const template = registerTemplate(renderButtonMenu);
freezeTemplate(renderButtonMenu);

class ButtonMenu extends LightningElement {
  constructor() {
    super();
    this.alternativeText = "Show menu";
    this.isOpen = false;
    this.hasRenderedMenu = false;
    this.alignment = "left";
    this.handleItemSelect = this.handleItemSelect.bind(this);
  }

  connectedCallback() {
    this.classList.add("slds-dropdown-trigger", "slds-dropdown-trigger_click");
    this.addEventListener("privatemenuselect", this.handleItemSelect);
  }

  disconnectedCallback() {
    this.removeEventListener("privatemenuselect", this.handleItemSelect);
  }

  get menuAlignment() {
    return this.alignment;
  }

  set menuAlignment(value) {
    this.alignment = typeof value === "string" && MENU_ALIGNMENTS.has(value) ? value : "left";
  }

  get dropdownClass() {
    const alignment = this.menuAlignment === "auto" ? "left" : this.menuAlignment;
    return "slds-dropdown " + alignment.split("-").map(part => "slds-dropdown_" + part).join(" ");
  }

  handleToggle() {
    if (this.disabled) return;
    this.setOpen(!this.isOpen);
  }

  setOpen(open) {
    if (this.isOpen === open) return;
    this.isOpen = open;
    if (open) this.hasRenderedMenu = true;
    this.classList.toggle("slds-is-open", open);
    // control_legacy_menu_navigation: close precedes the cancelable select.
    this.dispatchEvent(new CustomEvent(open ? "open" : "close"));
  }

  handleItemSelect(event) {
    event.stopPropagation();
    if (!this.isOpen || this.disabled) return;
    this.setOpen(false);
    this.dispatchEvent(new CustomEvent("select", {
      cancelable: true,
      detail: { value: event.detail.value },
    }));
    this.template.querySelector("button")?.focus();
  }
}

registerDecorators(ButtonMenu, {
  publicProps: {
    alternativeText: { config: 0 },
    disabled: { config: 0 },
    label: { config: 0 },
    menuAlignment: { config: 3 },
    title: { config: 0 },
    value: { config: 0 },
    variant: { config: 0 },
  },
  fields: ["isOpen", "hasRenderedMenu", "alignment"],
});
export default registerComponent(ButtonMenu, {
  tmpl: template,
  sel: "lightning-button-menu",
  apiVersion: 63,
});
