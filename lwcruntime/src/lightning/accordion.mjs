import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";

function renderAccordion($api, $cmp, $slotset) {
  return [$api.h("div", { key: 0 }, [$api.s("", { key: 1 }, [], $slotset)])];
}
renderAccordion.stylesheets = [];
renderAccordion.slots = [""];
const template = registerTemplate(renderAccordion);
freezeTemplate(renderAccordion);

class Accordion extends LightningElement {
  constructor() {
    super();
    this.sections = new Set();
    this.selectedSection = null;
    this.pendingSectionName = undefined;
    this.registerSection = this.registerSection.bind(this);
    this.activateSection = this.activateSection.bind(this);
  }

  connectedCallback() {
    this.classList.add("slds-accordion");
    this.setAttribute("role", "list");
    this.addEventListener("privateaccordionsectionregister", this.registerSection);
    this.addEventListener("privateaccordionsectionselect", this.activateSection);
  }

  disconnectedCallback() {
    this.removeEventListener("privateaccordionsectionregister", this.registerSection);
    this.removeEventListener("privateaccordionsectionselect", this.activateSection);
    this.sections.clear();
    this.selectedSection = null;
  }

  get activeSectionName() {
    return this.selectedSection?.getName();
  }

  set activeSectionName(value) {
    // The captured single-section mode ignores empty and non-string values.
    if (typeof value !== "string" || !value) return;
    const section = [...this.sections].find(section => section.getName() === value);
    if (section) {
      this.selectSection(section, true);
    } else if (!this.sections.size) {
      this.pendingSectionName = value;
    }
  }

  registerSection(event) {
    event.stopPropagation();
    const section = event.detail;
    if (!section || typeof section.getName !== "function" || typeof section.setOpen !== "function") return;
    this.sections.add(section);
    section.unregister = () => this.sections.delete(section);
    const requested = typeof this.pendingSectionName === "string" && section.getName() === this.pendingSectionName;
    if (!this.selectedSection || requested) {
      this.selectSection(section, false);
      if (requested) this.pendingSectionName = undefined;
    } else {
      section.setOpen(false);
    }
  }

  activateSection(event) {
    event.stopPropagation();
    if (this.sections.has(event.detail)) this.selectSection(event.detail, true);
  }

  selectSection(selected, notify) {
    if (this.selectedSection === selected) return;
    this.selectedSection = selected;
    for (const section of this.sections) section.setOpen(section === selected);
    if (notify) {
      this.dispatchEvent(new CustomEvent("sectiontoggle", {
        detail: { openSections: this.activeSectionName },
      }));
    }
  }
}

registerDecorators(Accordion, { publicProps: { activeSectionName: { config: 3 } } });
export default registerComponent(Accordion, {
  tmpl: template,
  sel: "lightning-accordion",
  apiVersion: 63,
});
