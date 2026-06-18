import _implicitStylesheets from "./badge.css.js";
import _implicitScopedStylesheets from "./badge.scoped.css.js";
import {freezeTemplate, registerTemplate} from "lwc";
function tmpl($api, $cmp, $slotset, $ctx) {
  const {d: api_dynamic_text, t: api_text} = $api;
  return [api_text(api_dynamic_text($cmp.label))];
  /*LWC compiler v8.20.4*/
}
export default registerTemplate(tmpl);
tmpl.stylesheets = [];
tmpl.stylesheetToken = "lwc-54j1ecj2k1t";
tmpl.legacyStylesheetToken = "lightning-badge_badge";
if (_implicitStylesheets) {
  tmpl.stylesheets.push.apply(tmpl.stylesheets, _implicitStylesheets);
}
if (_implicitScopedStylesheets) {
  tmpl.stylesheets.push.apply(tmpl.stylesheets, _implicitScopedStylesheets);
}
freezeTemplate(tmpl);
