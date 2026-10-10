import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import PrimitiveIcon from "./displayPrimitiveIcon.mjs";
import { computeSldsClass, getCategory, isValidName } from "./iconUtils.mjs";

const categories = new Set(["action", "custom", "doctype", "standard", "utility"]);

function renderIcon($api, $cmp) {
  const { h, t, c } = $api;
  const iconName = $cmp.resolvedIconName;
  const children = [c("lightning-primitive-icon", PrimitiveIcon, {
    key: 1,
    props: {
      iconName,
      svgClass: iconName && getCategory(iconName) === "utility"
        ? "slds-icon slds-icon-text-default" : "slds-icon",
    },
  }, [])];
  if ($cmp.alternativeText != null && $cmp.alternativeText !== "") {
    children.push(h("span", { key: 2, className: "slds-assistive-text" }, [
      t(String($cmp.alternativeText)),
    ]));
  }
  return [h("span", { key: 0 }, children)];
}
renderIcon.stylesheets = [];
const template = registerTemplate(renderIcon);
freezeTemplate(template);

class Icon extends LightningElement {
  get iconName() {
    return this._iconName;
  }

  set iconName(value) {
    this._iconName = value;
    if (this.isConnected) this.updateHostClasses();
  }

  get resolvedIconName() {
    return typeof this.iconName === "string" && isValidName(this.iconName)
      && categories.has(getCategory(this.iconName)) ? this.iconName : undefined;
  }

  connectedCallback() {
    this.updateHostClasses();
  }

  updateHostClasses() {
    this.classList.add("slds-icon_container");
    if (this._iconClass) this.classList.remove(this._iconClass);
    this._iconClass = typeof this.iconName === "string" && isValidName(this.iconName)
      ? computeSldsClass(this.iconName) : undefined;
    if (this._iconClass) this.classList.add(this._iconClass);
  }
}
registerDecorators(Icon, {
  publicProps: {
    iconName: { config: 3 },
    alternativeText: { config: 0 },
    size: { config: 0 },
    variant: { config: 0 },
    src: { config: 0 },
  },
  fields: ["_iconName"],
});
export default registerComponent(Icon, {
  tmpl: template,
  sel: "lightning-icon",
  apiVersion: 63,
});
