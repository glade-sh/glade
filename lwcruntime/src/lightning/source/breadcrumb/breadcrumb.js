import { registerDecorators as _registerDecorators, LightningElement, registerComponent as _registerComponent } from "lwc";
import _tmpl from "./breadcrumb.html.js";
/**
 * An item in the hierarchy path of the page the user is on.
 */
class LightningBreadcrumb extends LightningElement {
  constructor(...args) {
    super(...args);
    /**
     * The URL of the page that the breadcrumb goes to.
     * @type {string}
     */
    this.href = void 0;
    /**
     * The text label for the breadcrumb.
     * @type {string}
     * @required
     */
    this.label = void 0;
    /**
     * The name for the breadcrumb component. This value is optional and can be
     * used to identify the breadcrumb in a callback.
     *
     * @type {string}
     */
    this.name = void 0;
  }
  connectedCallback() {
    // add default CSS classes to custom element tag
    this.classList.add('slds-breadcrumb__item');
    this.classList.add('slds-text-title_caps');
    this.setAttribute('role', 'listitem');
  }
  /*LWC compiler v8.20.4*/
}
_registerDecorators(LightningBreadcrumb, {
  publicProps: {
    href: {
      config: 0
    },
    label: {
      config: 0
    },
    name: {
      config: 0
    }
  }
});
const __lwc_component_class_internal = _registerComponent(LightningBreadcrumb, {
  tmpl: _tmpl,
  sel: "lightning-breadcrumb",
  apiVersion: 63
});
export default __lwc_component_class_internal;