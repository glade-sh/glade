import { registerDecorators as _registerDecorators, LightningElement, registerComponent as _registerComponent } from "lwc";
import _tmpl from "./menuSubheader.html.js";
class LightningMenuSubheader extends LightningElement {
  constructor(...args) {
    super(...args);
    this.label = void 0;
  }
  connectedCallback() {
    // add default CSS classes to custom element tag
    this.classList.add('slds-dropdown__header');
    this.classList.add('slds-truncate');
    this.setAttribute('role', 'separator');
  }
  /*LWC compiler v8.20.4*/
}
_registerDecorators(LightningMenuSubheader, {
  publicProps: {
    label: {
      config: 0
    }
  }
});
const __lwc_component_class_internal = _registerComponent(LightningMenuSubheader, {
  tmpl: _tmpl,
  sel: "lightning-menu-subheader",
  apiVersion: 63
});
export default __lwc_component_class_internal;