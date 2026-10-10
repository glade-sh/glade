import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import { getDateTimeFormat } from "./i18nService.mjs";

const TIME_PATTERN = /^([01]\d|2[0-3]):([0-5]\d):([0-5]\d)(?:\.(\d{1,3}))?Z?$/;

function renderFormattedTime($api, $cmp) {
  if ($cmp.rawValue == null || $cmp.rawValue === "") return [];
  let text = String($cmp.rawValue);
  if ($cmp.value != null) {
    const parts = TIME_PATTERN.exec($cmp.value);
    const milliseconds = Number((parts[4] || "").padEnd(3, "0"));
    const date = new Date(Date.UTC(1970, 0, 1,
      Number(parts[1]), Number(parts[2]), Number(parts[3]), milliseconds));
    text = getDateTimeFormat({
      hour: "numeric", minute: "2-digit", second: "2-digit", timeZone: "UTC",
    }).format(date);
  }
  return [$api.t(text)];
}
renderFormattedTime.stylesheets = [];
const template = registerTemplate(renderFormattedTime);
freezeTemplate(template);

class FormattedTime extends LightningElement {
  get value() {
    return this.timeValue;
  }

  set value(value) {
    this.rawValue = value;
    this.timeValue = typeof value === "string" && TIME_PATTERN.test(value) ? value : null;
  }
}
registerDecorators(FormattedTime, {
  publicProps: { value: { config: 3 } },
  fields: ["rawValue", "timeValue"],
});
export default registerComponent(FormattedTime, {
  tmpl: template,
  sel: "lightning-formatted-time",
  apiVersion: 63,
});
