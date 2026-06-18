import _implicitStylesheets from "./menuSubheader.css.js";
import _implicitScopedStylesheets from "./menuSubheader.scoped.css.js";
import {freezeTemplate, parseFragment, registerTemplate} from "lwc";
const $fragment1 = parseFragment`<span class="slds-text-title_caps${0}"${2}>${"t1"}</span>`;
function tmpl($api, $cmp, $slotset, $ctx) {
  const {d: api_dynamic_text, sp: api_static_part, st: api_static_fragment} = $api;
  return [api_static_fragment($fragment1, 1, [api_static_part(1, null, api_dynamic_text($cmp.label))])];
  /*LWC compiler v8.20.4*/
}
export default registerTemplate(tmpl);
tmpl.stylesheets = [];
tmpl.stylesheetToken = "lwc-60src6kcqg2";
tmpl.legacyStylesheetToken = "lightning-menuSubheader_menuSubheader";
if (_implicitStylesheets) {
  tmpl.stylesheets.push.apply(tmpl.stylesheets, _implicitStylesheets);
}
if (_implicitScopedStylesheets) {
  tmpl.stylesheets.push.apply(tmpl.stylesheets, _implicitScopedStylesheets);
}
freezeTemplate(tmpl);
