import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import DisplayPrimitiveIcon from "./displayPrimitiveIcon.mjs";

function renderCarousel($api, $cmp, $slotset) {
  const { h, c, s, i, k, b, t } = $api;
  const autoplay = $cmp.disableAutoScroll ? null : h("span", {
    key: 2,
    className: "slds-carousel__autoplay",
  }, [h("button", {
    key: 3,
    className: "slds-button slds-button_icon slds-button_icon-border-filled slds-button_icon-x-small",
    attrs: { title: "Stop auto-play" },
  }, [
    c("lightning-primitive-icon", DisplayPrimitiveIcon, {
      key: 4,
      className: "slds-button__icon",
      props: {
        svgClass: "slds-icon slds-icon-text-default slds-icon_x-small",
        svgPath: "M100 80h100v360H100zM320 80h100v360H320z",
      },
    }, []),
    h("span", { key: 5, className: "slds-assistive-text" }, [t("Stop auto-play")]),
  ])]);
  const indicators = i($cmp.images, (_image, index) => {
    const selected = index === $cmp.selectedIndex;
    return h("li", {
      key: k(9, index),
      className: "slds-carousel__indicator",
      attrs: { role: "presentation" },
    }, [h("a", {
      key: 10,
      className: "slds-carousel__indicator-action" + (selected ? " slds-is-active" : ""),
      attrs: {
        href: "about:blank",
        role: "tab",
        tabindex: selected ? "0" : "-1",
        "aria-selected": String(selected),
        "data-index": String(index),
      },
      on: { click: b($cmp.handleSelect) },
    }, [h("span", { key: 11, className: "slds-assistive-text" }, [])])]);
  });
  return [h("div", { key: 0, className: "slds-carousel" }, [
    h("div", { key: 1, className: "slds-carousel__stage" }, [
      autoplay,
      h("div", { key: 6, className: "slds-carousel__panels" }, [
        s("", { key: 7 }, [], $slotset),
      ]),
      h("ul", {
        key: 8,
        className: "slds-carousel__indicators",
        attrs: { role: "tablist" },
      }, indicators),
    ]),
  ])];
}
renderCarousel.stylesheets = [];
renderCarousel.slots = [""];
const template = registerTemplate(renderCarousel);
freezeTemplate(template);

class Carousel extends LightningElement {
  constructor() {
    super();
    this.images = [];
    this.selectedIndex = 0;
    this._disableAutoScroll = false;
    this.registerImage = this.registerImage.bind(this);
  }

  connectedCallback() {
    this.addEventListener("privatecarouselimageregister", this.registerImage);
  }

  disconnectedCallback() {
    this.removeEventListener("privatecarouselimageregister", this.registerImage);
    this.images = [];
  }

  get disableAutoScroll() {
    return this._disableAutoScroll;
  }

  set disableAutoScroll(value) {
    this._disableAutoScroll = typeof value === "string" || Boolean(value);
  }

  registerImage(event) {
    event.stopPropagation();
    const image = event.detail;
    if (!image || typeof image.setSelected !== "function" || this.images.includes(image)) return;
    this.images = [...this.images, image];
    image.unregister = () => {
      this.images = this.images.filter(item => item !== image);
      this.selectImage(Math.min(this.selectedIndex, Math.max(0, this.images.length - 1)));
    };
    this.selectImage(this.selectedIndex);
  }

  handleSelect(event) {
    event.preventDefault();
    const index = Number(event.currentTarget.getAttribute("data-index"));
    if (Number.isInteger(index) && index >= 0 && index < this.images.length) this.selectImage(index);
  }

  selectImage(index) {
    this.selectedIndex = index;
    this.images.forEach((image, imageIndex) => image.setSelected(imageIndex === index));
  }
}
registerDecorators(Carousel, {
  publicProps: { disableAutoScroll: { config: 3 } },
  fields: ["images", "selectedIndex", "_disableAutoScroll"],
});
export default registerComponent(Carousel, {
  tmpl: template,
  sel: "lightning-carousel",
  apiVersion: 63,
});
