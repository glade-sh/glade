import { freezeTemplate, registerComponent, registerTemplate } from "lwc";
import LightningBadge from "./source/badge/badge.js";

function renderBadge($api, $cmp) {
  const label = $cmp.label == null ? "" : String($cmp.label);
  return [$api.h("span", { key: 0 }, [$api.t(label)])];
}
renderBadge.stylesheets = [];
const template = registerTemplate(renderBadge);
freezeTemplate(template);

class Badge extends LightningBadge {}
export default registerComponent(Badge, {
  tmpl: template,
  sel: "lightning-badge",
  apiVersion: 63,
});
