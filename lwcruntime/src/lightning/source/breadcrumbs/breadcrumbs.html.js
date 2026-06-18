import _implicitStylesheets from "./breadcrumbs.css.js";
import _implicitScopedStylesheets from "./breadcrumbs.scoped.css.js";
import {freezeTemplate, registerTemplate} from "lwc";
const stc0 = {
  classMap: {
    "slds-breadcrumb": true,
    "slds-list_horizontal": true,
    "slds-wrap": true
  },
  attrs: {
    "role": "list"
  },
  key: 0
};
const stc1 = {
  key: 1
};
const stc2 = [];
function tmpl($api, $cmp, $slotset, $ctx) {
  const {s: api_slot, h: api_element} = $api;
  return [api_element("div", stc0, [api_slot("", stc1, stc2, $slotset)])];
  /*LWC compiler v8.20.4*/
}
export default registerTemplate(tmpl);
tmpl.slots = [""];
tmpl.stylesheets = [];
tmpl.stylesheetToken = "lwc-1d8f8btsns8";
tmpl.legacyStylesheetToken = "lightning-breadcrumbs_breadcrumbs";
if (_implicitStylesheets) {
  tmpl.stylesheets.push.apply(tmpl.stylesheets, _implicitStylesheets);
}
if (_implicitScopedStylesheets) {
  tmpl.stylesheets.push.apply(tmpl.stylesheets, _implicitScopedStylesheets);
}
freezeTemplate(tmpl);
