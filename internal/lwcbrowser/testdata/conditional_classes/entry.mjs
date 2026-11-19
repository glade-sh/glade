import { createElement } from "lwc";
import Input from "lightning/input";
import Textarea from "lightning/textarea";
import Combobox from "lightning/combobox";
import RadioGroup from "lightning/radioGroup";
import ButtonStateful from "lightning/buttonStateful";

// The same probes as probe.html, created directly.
const options = [{ label: "Alpha", value: "a" }, { label: "Beta", value: "b" }];
const probes = [
  ["input_valid", "lightning-input", Input, { label: "Valid input", value: "x" }],
  ["input_error", "lightning-input", Input, { label: "Required input", required: true }, true],
  ["textarea_valid", "lightning-textarea", Textarea, { label: "Notes", value: "x" }],
  ["textarea_error", "lightning-textarea", Textarea, { label: "Required notes", required: true }, true],
  ["combobox_closed", "lightning-combobox", Combobox, { label: "Pick", options, value: "a" }],
  ["combobox_open", "lightning-combobox", Combobox, { label: "Pick open", options, value: "a" }],
  ["combobox_error", "lightning-combobox", Combobox, { label: "Pick required", options, required: true }, true],
  ["radio_valid", "lightning-radio-group", RadioGroup, { label: "Radio", options, value: "a" }],
  ["radio_error", "lightning-radio-group", RadioGroup, { label: "Radio required", options, required: true }, true],
  ["stateful_selected", "lightning-button-stateful", ButtonStateful, { labelWhenOff: "Off", labelWhenOn: "On", selected: true }],
  ["stateful_unselected", "lightning-button-stateful", ButtonStateful, { labelWhenOff: "Off", labelWhenOn: "On", selected: false }],
];
const report = [];
for (const [id, tag, Ctor, props, reportValidity] of probes) {
  const wrapper = document.createElement("div");
  wrapper.dataset.probe = id;
  const element = createElement(tag, { is: Ctor });
  Object.assign(element, props);
  wrapper.appendChild(element);
  document.body.appendChild(wrapper);
  if (reportValidity) report.push(element);
}
await Promise.resolve();
for (const element of report) element.reportValidity();
await new Promise((resolve) => setTimeout(resolve, 0));
window.__probesReady = true;
