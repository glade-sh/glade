import { registerComponent } from "lwc";
import LightningVerticalNavigationItem from "./source/verticalNavigationItem/verticalNavigationItem.js";
import template from "./source/verticalNavigationItem/verticalNavigationItem.html.js";

class VerticalNavigationItem extends LightningVerticalNavigationItem {
  constructor(...args) {
    super(...args);
    this.href = "javascript:void(0)";
  }

  handleClick(event) {
    super.handleClick(event);
    if (this.href === "javascript:void(0)") event.preventDefault();
  }
}

export default registerComponent(VerticalNavigationItem, {
  tmpl: template,
  sel: "lightning-vertical-navigation-item",
  apiVersion: 63,
});
