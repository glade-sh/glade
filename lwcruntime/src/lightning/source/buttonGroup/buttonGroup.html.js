import _implicitStylesheets from "./buttonGroup.css.js";
import _implicitScopedStylesheets from "./buttonGroup.scoped.css.js";
import {freezeTemplate, registerTemplate} from "lwc";
const stc0 = [];
function tmpl($api, $cmp, $slotset, $ctx) {
  const {b: api_bind, s: api_slot} = $api;
  const {_m0} = $ctx;
  return [api_slot("", {
    key: 0,
    on: _m0 || ($ctx._m0 = {
      "slotchange": api_bind($cmp.handleSlotChange)
    })
  }, stc0, $slotset)];
  /*LWC compiler v8.20.4*/
}
export default registerTemplate(tmpl);
tmpl.slots = [""];
tmpl.stylesheets = [];
tmpl.stylesheetToken = "lwc-6o7fbml4rr3";
tmpl.legacyStylesheetToken = "lightning-buttonGroup_buttonGroup";
if (_implicitStylesheets) {
  tmpl.stylesheets.push.apply(tmpl.stylesheets, _implicitStylesheets);
}
if (_implicitScopedStylesheets) {
  tmpl.stylesheets.push.apply(tmpl.stylesheets, _implicitScopedStylesheets);
}
freezeTemplate(tmpl);
