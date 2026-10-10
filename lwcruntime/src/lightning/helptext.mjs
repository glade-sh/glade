import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import ButtonIcon from "./buttonIcon.mjs";

function renderHelptext($api, $cmp) {
  return [$api.h("div", { key: 0, className: "slds-form-element__icon" }, [
    $api.c("lightning-button-icon", ButtonIcon, {
      key: 1,
      props: { iconName: $cmp.iconName || "utility:info", variant: "bare", alternativeText: "Help", title: null },
    }, []),
  ])];
}
renderHelptext.stylesheets = [];
const template = registerTemplate(renderHelptext);
freezeTemplate(template);

class Helptext extends LightningElement {}
registerDecorators(Helptext, {
  publicProps: { content: { config: 0 }, iconName: { config: 0 } },
});
export default registerComponent(Helptext, {
  tmpl: template,
  sel: "lightning-helptext",
  apiVersion: 63,
});
