import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import PrimitiveIcon from "./displayPrimitiveIcon.mjs";

function normalizeChoice(value, choices, fallback) {
  const normalized = String(value || fallback).trim().toLowerCase();
  return choices.includes(normalized) ? normalized : fallback;
}

function buttonIconClassMap(variant, size) {
  const normalizedVariant = normalizeChoice(variant, ["bare", "brand", "container", "border", "border-filled", "bare-inverse", "border-inverse"], "border");
  const normalizedSize = normalizeChoice(size, ["xx-small", "x-small", "small", "medium", "large"], "medium");
  const isBare = normalizedVariant.startsWith("bare");
  return {
    "slds-button": true,
    "slds-button_icon": true,
    "slds-button_icon-bare": isBare,
    "slds-button_icon-container": normalizedVariant === "container",
    "slds-button_icon-border": normalizedVariant === "border",
    "slds-button_icon-border-filled": normalizedVariant === "border-filled",
    "slds-button_icon-border-inverse": normalizedVariant === "border-inverse",
    "slds-button_icon-inverse": normalizedVariant === "bare-inverse",
    "slds-button_icon-brand": normalizedVariant === "brand",
    "slds-button_icon-small": !isBare && normalizedSize === "small",
    "slds-button_icon-x-small": !isBare && normalizedSize === "x-small",
    "slds-button_icon-xx-small": !isBare && normalizedSize === "xx-small",
  };
}

function renderButtonIcon($api, $cmp) {
  const { h, t, c } = $api;
  const classes = buttonIconClassMap($cmp.variant, $cmp.size);
  const children = [c("lightning-primitive-icon", PrimitiveIcon, {
    key: 1,
    props: { iconName: $cmp.iconName, svgClass: "slds-button__icon" },
  }, [])];
  if ($cmp.alternativeText != null && $cmp.alternativeText !== "") {
    children.push(h("span", { key: 2, className: "slds-assistive-text" }, [
      t(String($cmp.alternativeText)),
    ]));
  }
  return [h("button", {
    key: 0,
    className: Object.keys(classes).filter(name => classes[name]).join(" "),
    attrs: {
      type: normalizeChoice($cmp.type, ["button", "reset", "submit"], "button"),
      name: $cmp.name || undefined,
      value: $cmp.value == null ? undefined : String($cmp.value),
      title: $cmp.computedTitle,
      tabindex: $cmp.tabIndex,
    },
    props: { disabled: Boolean($cmp.disabled) },
  }, children)];
}
renderButtonIcon.stylesheets = [];
const template = registerTemplate(renderButtonIcon);
freezeTemplate(template);

class ButtonIcon extends LightningElement {
  get computedTitle() {
    const value = this.title === undefined ? this.alternativeText : this.title;
    return value == null || value === "" ? null : String(value);
  }

  focus() {
    this.template.querySelector("button")?.focus();
  }

  blur() {
    this.template.querySelector("button")?.blur();
  }

  click() {
    this.template.querySelector("button")?.click();
  }
}
registerDecorators(ButtonIcon, {
  publicProps: {
    iconName: { config: 0 },
    alternativeText: { config: 0 },
    variant: { config: 0 },
    size: { config: 0 },
    type: { config: 0 },
    name: { config: 0 },
    value: { config: 0 },
    title: { config: 0 },
    disabled: { config: 0 },
    tabIndex: { config: 0 },
  },
  publicMethods: ["focus", "blur", "click"],
});
export default registerComponent(ButtonIcon, {
  tmpl: template,
  sel: "lightning-button-icon",
  apiVersion: 63,
});
