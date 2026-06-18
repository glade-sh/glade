import _implicitStylesheets from "./breadcrumb.css.js";
import _implicitScopedStylesheets from "./breadcrumb.scoped.css.js";
import {freezeTemplate, parseFragment, registerTemplate} from "lwc";
const $fragment1 = parseFragment`<a${"a0:href"}${3}>${"t1"}</a>`;
function tmpl($api, $cmp, $slotset, $ctx) {
  const {d: api_dynamic_text, sp: api_static_part, st: api_static_fragment} = $api;
  return [api_static_fragment($fragment1, 1, [api_static_part(0, {
    attrs: {
      "href": $cmp.href
    }
  }, null), api_static_part(1, null, api_dynamic_text($cmp.label))])];
  /*LWC compiler v8.20.4*/
}
export default registerTemplate(tmpl);
tmpl.stylesheets = [];
tmpl.stylesheetToken = "lwc-76a30tv0gqp";
tmpl.legacyStylesheetToken = "lightning-breadcrumb_breadcrumb";
if (_implicitStylesheets) {
  tmpl.stylesheets.push.apply(tmpl.stylesheets, _implicitStylesheets);
}
if (_implicitScopedStylesheets) {
  tmpl.stylesheets.push.apply(tmpl.stylesheets, _implicitScopedStylesheets);
}
freezeTemplate(tmpl);
