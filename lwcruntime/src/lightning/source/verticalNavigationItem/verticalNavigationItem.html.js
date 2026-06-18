import _implicitStylesheets from "./verticalNavigationItem.css.js";
import _implicitScopedStylesheets from "./verticalNavigationItem.scoped.css.js";
import {freezeTemplate, parseFragment, registerTemplate} from "lwc";
const $fragment1 = parseFragment`<a${"a0:href"} class="slds-nav-vertical__action${0}"${"a0:aria-current"}${2}>${"t1"}</a>`;
function tmpl($api, $cmp, $slotset, $ctx) {
  const {b: api_bind, d: api_dynamic_text, sp: api_static_part, st: api_static_fragment} = $api;
  const {_m0, _m1} = $ctx;
  return [api_static_fragment($fragment1, 1, [api_static_part(0, {
    on: _m1 || ($ctx._m1 = {
      "click": api_bind($cmp.handleClick)
    }),
    attrs: {
      "href": $cmp.href,
      "aria-current": $cmp.ariaCurrent
    }
  }, null), api_static_part(1, null, api_dynamic_text($cmp.label))])];
  /*LWC compiler v8.20.4*/
}
export default registerTemplate(tmpl);
tmpl.stylesheets = [];
tmpl.stylesheetToken = "lwc-1o18b5387u6";
tmpl.legacyStylesheetToken = "lightning-verticalNavigationItem_verticalNavigationItem";
if (_implicitStylesheets) {
  tmpl.stylesheets.push.apply(tmpl.stylesheets, _implicitStylesheets);
}
if (_implicitScopedStylesheets) {
  tmpl.stylesheets.push.apply(tmpl.stylesheets, _implicitScopedStylesheets);
}
freezeTemplate(tmpl);
