import { createBaseComponent } from "./base.mjs";
import PrimitiveIcon from "./displayPrimitiveIcon.mjs";

// Path from the bundled SLDS utility/email.svg.
const emailPath = "M249 301c6 6 15 6 21 0L496 91c4-8 3-21-13-21L36 71c-12 0-22 11-13 21zm251-128c0-10-12-16-20-9L303 327c-12 11-27 17-43 17s-31-6-43-16L41 164c-8-7-20-2-20 9-1-3-1 227-1 227a40 40 0 0 0 40 40h400a40 40 0 0 0 40-40z";

function renderFormattedEmail($api, $cmp) {
  if (typeof $cmp.value !== "string" || !$cmp.value) return [];
  const { h, t, c } = $api;
  return [h("a", { key: 0, attrs: { href: `mailto:${$cmp.value}` } }, [
    c("lightning-primitive-icon", PrimitiveIcon, {
      key: 1,
      className: "slds-m-right_xx-small",
      props: {
        svgClass: "slds-icon slds-icon-text-default slds-icon_x-small",
        svgPath: emailPath,
      },
    }, []),
    h("span", { key: 2, className: "slds-assistive-text" }, [t("Email")]),
    t($cmp.label || $cmp.value),
  ])];
}

export default createBaseComponent("lightning-formatted-email", renderFormattedEmail);
