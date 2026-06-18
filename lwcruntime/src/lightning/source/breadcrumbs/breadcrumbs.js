import _tmpl from "./breadcrumbs.html.js";
import { LightningElement, registerComponent as _registerComponent } from 'lwc';

/**
 * A hierarchy path of the page you're currently visiting within the website or app.
 */
class LightningBreadcrumbs extends LightningElement {
  connectedCallback() {
    this.setAttribute('aria-label', 'Breadcrumbs');
    this.setAttribute('role', 'navigation');
  }
  /*LWC compiler v8.20.4*/
}
const __lwc_component_class_internal = _registerComponent(LightningBreadcrumbs, {
  tmpl: _tmpl,
  sel: "lightning-breadcrumbs",
  apiVersion: 63
});
export default __lwc_component_class_internal;