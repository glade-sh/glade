import { registerComponent } from "lwc";
import LightningBreadcrumb from "./source/breadcrumb/breadcrumb.js";
import template from "./source/breadcrumb/breadcrumb.html.js";

class Breadcrumb extends LightningBreadcrumb {
  connectedCallback() {
    const suppliedCapsClass = this.classList.contains("slds-text-title_caps");
    super.connectedCallback();
    if (!suppliedCapsClass) this.classList.remove("slds-text-title_caps");
  }
}

export default registerComponent(Breadcrumb, {
  tmpl: template,
  sel: "lightning-breadcrumb",
  apiVersion: 63,
});
