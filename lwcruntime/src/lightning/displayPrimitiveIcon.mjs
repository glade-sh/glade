import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import { getIconPath, isValidName } from "./iconUtils.mjs";

// Bundled SLDS glyphs shared by display icons, button-icon, pill and helptext.
const glyphs = {
  "utility:check": "M191 425 26 259c-6-6-6-16 0-22l22-22c6-6 16-6 22 0l124 125a10 10 0 0 0 15 0L452 95c6-6 16-6 22 0l22 22c6 6 6 16 0 22L213 425c-6 7-16 7-22 0",
  "utility:close": "m310 254 130-131c6-6 6-15 0-21l-20-21c-6-6-15-6-21 0L268 212a10 10 0 0 1-14 0L123 80c-6-6-15-6-21 0l-21 21c-6 6-6 15 0 21l131 131c4 4 4 10 0 14L80 399c-6 6-6 15 0 21l21 21c6 6 15 6 21 0l131-131a10 10 0 0 1 14 0l131 131c6 6 15 6 21 0l21-21c6-6 6-15 0-21L310 268a10 10 0 0 1 0-14",
  "utility:info": "M260 20a240 240 0 1 0 0 480 240 240 0 1 0 0-480m0 121c17 0 30 13 30 30s-13 30-30 30-30-13-30-30 13-30 30-30m50 210c0 5-4 9-10 9h-80c-5 0-10-3-10-9v-20c0-5 4-11 10-11 5 0 10-3 10-9v-40c0-5-4-11-10-11-5 0-10-3-10-9v-20c0-5 4-11 10-11h60c5 0 10 5 10 11v80c0 5 4 9 10 9 5 0 10 5 10 11z",
};

function renderIconGlyph($api, $cmp) {
  const { h } = $api;
  const path = $cmp.svgPath || (typeof $cmp.iconName === "string" && Object.hasOwn(glyphs, $cmp.iconName)
    ? glyphs[$cmp.iconName] : undefined);
  const contents = path ? [h("g", { key: 1, svg: true }, [
    h("path", { key: 2, svg: true, attrs: { d: path } }, []),
  ])] : [h("use", {
    key: 3,
    svg: true,
    attrs: { href: isValidName($cmp.iconName) ? getIconPath($cmp.iconName) : "" },
  }, [])];
  return [h("svg", {
    key: 0,
    svg: true,
    className: $cmp.svgClass || "slds-icon",
    attrs: { "aria-hidden": "true", viewBox: "0 0 520 520", focusable: "false" },
  }, contents)];
}
renderIconGlyph.stylesheets = [];
const template = registerTemplate(renderIconGlyph);
freezeTemplate(template);

class DisplayPrimitiveIcon extends LightningElement {}
registerDecorators(DisplayPrimitiveIcon, {
  publicProps: {
    iconName: { config: 0 },
    svgClass: { config: 0 },
    svgPath: { config: 0 },
  },
});
export default registerComponent(DisplayPrimitiveIcon, {
  tmpl: template,
  sel: "lightning-primitive-icon",
  apiVersion: 63,
});
