import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import { getDateTimeFormat } from "./i18nService.mjs";

const DATE_OPTIONS = [
  "weekday", "era", "year", "month", "day", "hour", "minute", "second",
  "timeZone", "timeZoneName", "hour12",
];

function renderFormattedDateTime($api, $cmp) {
  const value = $cmp.value;
  if (value == null || value === "") return [];
  try {
    const epoch = typeof value === "string" && /^[+-]?\d+$/.test(value.trim());
    const date = new Date(epoch ? Number(value) : value);
    if (!Number.isFinite(date.getTime())) return [];
    const options = {};
    for (const name of DATE_OPTIONS) {
      if ($cmp[name] != null) options[name] = $cmp[name];
    }
    // control_legacy_default_date: no supplied format uses the medium date.
    if (Object.keys(options).length === 0) {
      Object.assign(options, { year: "numeric", month: "short", day: "numeric" });
    }
    return [$api.t(getDateTimeFormat(options).format(date))];
  } catch {
    return [];
  }
}
renderFormattedDateTime.stylesheets = [];
const template = registerTemplate(renderFormattedDateTime);
freezeTemplate(template);

class FormattedDateTime extends LightningElement {}
registerDecorators(FormattedDateTime, {
  publicProps: Object.fromEntries(["value", ...DATE_OPTIONS].map(name => [name, { config: 0 }])),
});
export default registerComponent(FormattedDateTime, {
  tmpl: template,
  sel: "lightning-formatted-date-time",
  apiVersion: 63,
});
