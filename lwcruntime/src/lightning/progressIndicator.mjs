import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import ProgressBar from "./progressBar.mjs";

function renderProgressIndicator($api, $cmp, $slotset) {
  const { h, s, c } = $api;
  return [h("div", { key: 0, className: "slds-progress" }, [
    h("ol", { key: 1, className: "slds-progress__list" }, [
      s("", { key: 2 }, [], $slotset),
    ]),
    c("lightning-progress-bar", ProgressBar, {
      key: 3,
      props: { value: $cmp.progressValue, size: "x-small" },
    }, []),
  ])];
}
renderProgressIndicator.stylesheets = [];
renderProgressIndicator.slots = [""];
const template = registerTemplate(renderProgressIndicator);
freezeTemplate(template);

class ProgressIndicator extends LightningElement {
  constructor() {
    super();
    this.steps = [];
    this.progressValue = 0;
    this.registerStep = this.registerStep.bind(this);
  }

  connectedCallback() {
    this.addEventListener("privateprogressstepregister", this.registerStep);
  }

  disconnectedCallback() {
    this.removeEventListener("privateprogressstepregister", this.registerStep);
    this.steps = [];
  }

  get currentStep() {
    return this._currentStep;
  }

  set currentStep(value) {
    this._currentStep = value;
    this.updateSteps();
  }

  registerStep(event) {
    event.stopPropagation();
    const step = event.detail;
    if (!step || typeof step.getValue !== "function" || typeof step.setState !== "function") return;
    if (!this.steps.includes(step)) this.steps.push(step);
    step.unregister = () => {
      this.steps = this.steps.filter(item => item !== step);
      this.updateSteps();
    };
    step.update = () => this.updateSteps();
    this.updateSteps();
  }

  updateSteps() {
    const selected = Math.max(0, this.steps.findIndex(step => step.getValue() === this.currentStep));
    this.steps.forEach((step, index) => {
      step.setState(index < selected ? "completed" : index === selected ? "active" : "incomplete");
    });
    this.progressValue = this.steps.length > 1 ? selected * 100 / (this.steps.length - 1) : 0;
  }
}
registerDecorators(ProgressIndicator, {
  publicProps: { currentStep: { config: 3 }, type: { config: 0 }, variant: { config: 0 }, hasError: { config: 0 } },
  fields: ["progressValue"],
});
export default registerComponent(ProgressIndicator, {
  tmpl: template,
  sel: "lightning-progress-indicator",
  apiVersion: 63,
});
