import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";

// Private section affordance, using the bundled SLDS utility chevrondown path.
function renderSectionIcon($api) {
  const { h } = $api;
  return [h("svg", {
    key: 0,
    svg: true,
    className: "slds-button__icon slds-button__icon_left slds-icon slds-icon-text-default slds-icon_x-small",
    attrs: { "aria-hidden": "true", viewBox: "0 0 520 520", focusable: "false" },
  }, [h("g", { key: 1, svg: true }, [h("path", {
    key: 2,
    svg: true,
    attrs: { d: "M476 178 271 385c-6 6-16 6-22 0L44 178c-6-6-6-16 0-22l22-22c6-6 16-6 22 0l161 163c6 6 16 6 22 0l161-162c6-6 16-6 22 0l22 22c5 6 5 15 0 21" },
  }, [])])])];
}
renderSectionIcon.stylesheets = [];
const iconTemplate = registerTemplate(renderSectionIcon);
freezeTemplate(renderSectionIcon);
class SectionIcon extends LightningElement {}
const AccordionSectionIcon = registerComponent(SectionIcon, {
  tmpl: iconTemplate,
  sel: "lightning-primitive-icon",
  apiVersion: 63,
});

function renderAccordionSection($api, $cmp, $slotset) {
  const { h, t, c, b, s } = $api;
  const label = $cmp.label == null ? "" : String($cmp.label);
  return [h("div", { key: 0, classMap: { "slds-accordion__list-item": true } }, [
    h("section", {
      key: 1,
      className: "slds-accordion__section" + ($cmp.isOpen ? " slds-is-open" : ""),
    }, [
      h("div", { key: 2, classMap: { "slds-accordion__summary": true } }, [
        h("h2", { key: 3, classMap: { "slds-accordion__summary-heading": true } }, [
          h("button", {
            key: 4,
            className: "section-control slds-button slds-button_reset slds-accordion__summary-action",
            attrs: { type: "button", "aria-expanded": String($cmp.isOpen) },
            on: { click: b($cmp.handleSelect) },
          }, [
            c("lightning-primitive-icon", AccordionSectionIcon, { key: 5 }, []),
            h("span", {
              key: 6,
              classMap: { "slds-accordion__summary-content": true },
              attrs: { title: $cmp.label == null ? null : label },
            }, [t(label)]),
          ]),
        ]),
        s("actions", { key: 7, attrs: { name: "actions" } }, [], $slotset),
      ]),
      h("div", {
        key: 8,
        classMap: { "slds-accordion__content": true },
        attrs: { "aria-hidden": String(!$cmp.isOpen) },
      }, [s("", { key: 9 }, [], $slotset)]),
    ]),
  ])];
}
renderAccordionSection.stylesheets = [];
renderAccordionSection.slots = ["", "actions"];
const template = registerTemplate(renderAccordionSection);
freezeTemplate(renderAccordionSection);

class AccordionSection extends LightningElement {
  constructor() {
    super();
    this.isOpen = false;
    this.registration = {
      getName: () => this.name,
      setOpen: open => { this.isOpen = open; },
    };
  }

  connectedCallback() {
    this.classList.add("slds-accordion__list-item");
    this.setAttribute("role", "listitem");
    this.dispatchEvent(new CustomEvent("privateaccordionsectionregister", {
      bubbles: true,
      composed: true,
      detail: this.registration,
    }));
  }

  disconnectedCallback() {
    this.registration.unregister?.();
  }

  handleSelect() {
    this.dispatchEvent(new CustomEvent("privateaccordionsectionselect", {
      bubbles: true,
      composed: true,
      detail: this.registration,
    }));
  }
}

registerDecorators(AccordionSection, {
  publicProps: { label: { config: 0 }, name: { config: 0 } },
  fields: ["isOpen"],
});
export default registerComponent(AccordionSection, {
  tmpl: template,
  sel: "lightning-accordion-section",
  apiVersion: 63,
});
