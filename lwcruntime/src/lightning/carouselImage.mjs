import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";

function renderCarouselImage($api, $cmp) {
  const { h, t } = $api;
  return [h("div", {
    key: 0,
    attrs: { role: "tabpanel", "aria-hidden": String(!$cmp.selected) },
  }, [h("a", {
    key: 1,
    className: "slds-carousel__panel-action slds-text-link_reset",
    attrs: { href: $cmp.href, tabindex: $cmp.selected ? "0" : "-1" },
  }, [
    h("div", { key: 2, className: "slds-carousel__image" }, [
      h("img", { key: 3, attrs: { src: $cmp.src, alt: $cmp.alternativeText } }, []),
    ]),
    h("div", { key: 4, className: "slds-carousel__content content-container" }, [
      h("h2", { key: 5, className: "slds-carousel__content-title" }, [
        t($cmp.header == null ? "" : String($cmp.header)),
      ]),
      h("p", { key: 6 }, [t($cmp.description == null ? "" : String($cmp.description))]),
    ]),
  ])])];
}
renderCarouselImage.stylesheets = [];
const template = registerTemplate(renderCarouselImage);
freezeTemplate(template);

class CarouselImage extends LightningElement {
  constructor() {
    super();
    this.selected = false;
    this.registration = { setSelected: selected => { this.selected = selected; } };
  }

  connectedCallback() {
    this.classList.add("slds-carousel__panel");
    this.dispatchEvent(new CustomEvent("privatecarouselimageregister", {
      bubbles: true,
      composed: true,
      detail: this.registration,
    }));
  }

  disconnectedCallback() {
    this.registration.unregister?.();
  }
}
registerDecorators(CarouselImage, {
  publicProps: {
    src: { config: 0 },
    header: { config: 0 },
    description: { config: 0 },
    alternativeText: { config: 0 },
    href: { config: 0 },
  },
  fields: ["selected"],
});
export default registerComponent(CarouselImage, {
  tmpl: template,
  sel: "lightning-carousel-image",
  apiVersion: 63,
});
