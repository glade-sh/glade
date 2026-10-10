import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";

function renderTile($api, $cmp, $slotset) {
  const label = $cmp.label == null ? "" : String($cmp.label);
  return [
    $api.h("h3", {
      key: 0,
      className: "slds-tile__title slds-truncate",
      attrs: { title: $cmp.label == null ? null : label },
    }, [$api.h("a", { key: 1, attrs: { href: $cmp.href } }, [$api.t(label)])]),
    $api.h("div", { key: 2, className: "slds-tile__detail" }, [
      $api.s("", { key: 3 }, [], $slotset),
    ]),
  ];
}
renderTile.stylesheets = [];
renderTile.slots = [""];
const template = registerTemplate(renderTile);
freezeTemplate(template);

class Tile extends LightningElement {
  connectedCallback() {
    this.classList.add("slds-tile");
  }
}

registerDecorators(Tile, {
  publicProps: { label: { config: 0 }, href: { config: 0 } },
});
export default registerComponent(Tile, {
  tmpl: template,
  sel: "lightning-tile",
  apiVersion: 63,
});
