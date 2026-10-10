import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import ButtonIcon from "./buttonIcon.mjs";

function renderPill($api, $cmp) {
  const { h, t, c, b } = $api;
  const label = $cmp.label == null ? "" : String($cmp.label);
  const removeLabel = "Remove " + label;
  return [h("span", { key: 0, className: "slds-pill" }, [
    h("span", {
      key: 1,
      className: "slds-pill__label",
      attrs: { title: $cmp.label == null ? null : label },
    }, [t(label)]),
    c("lightning-button-icon", ButtonIcon, {
      key: 2,
      className: "slds-pill__remove",
      props: { iconName: "utility:close", variant: "bare", alternativeText: removeLabel, title: removeLabel },
      on: { click: b($cmp.handleRemove) },
    }, []),
  ])];
}
renderPill.stylesheets = [];
const template = registerTemplate(renderPill);
freezeTemplate(template);

class Pill extends LightningElement {
  handleRemove() {
    this.dispatchEvent(new CustomEvent("remove", {
      detail: { name: this.name },
      cancelable: true,
    }));
  }
}
registerDecorators(Pill, {
  publicProps: { label: { config: 0 }, name: { config: 0 }, href: { config: 0 } },
});
export default registerComponent(Pill, {
  tmpl: template,
  sel: "lightning-pill",
  apiVersion: 63,
});
