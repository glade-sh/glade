import { registerDecorators as _registerDecorators, LightningElement, registerComponent as _registerComponent } from "lwc";
import _tmpl from "./badge.html.js";
/**
 * Represents a label which holds a small amount of information, such as the
 * number of unread notifications.
 */
class LightningBadge extends LightningElement {
  constructor(...args) {
    super(...args);
    /**
     * The text to be displayed inside the badge.
     *
     * @type {string}
     * @required
     */
    this.label = void 0;
  }
  connectedCallback() {
    this.classList.add('slds-badge');
  }
  /*LWC compiler v8.20.4*/
}
_registerDecorators(LightningBadge, {
  publicProps: {
    label: {
      config: 0
    }
  }
});
const __lwc_component_class_internal = _registerComponent(LightningBadge, {
  tmpl: _tmpl,
  sel: "lightning-badge",
  apiVersion: 63
});
export default __lwc_component_class_internal;