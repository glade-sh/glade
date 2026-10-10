import { createBaseComponent } from "./base.mjs";

function renderFormattedAddress($api, $cmp) {
  const lines = [$cmp.street, $cmp.city, $cmp.province, $cmp.postalCode, $cmp.country]
    .filter(value => value != null && value !== "")
    .flatMap(value => String(value).split(/\r?\n/));
  const address = lines.join("\n");
  return [$api.h("a", {
    key: 0,
    attrs: {
      href: `https://www.google.com/maps?q=${encodeURIComponent(address)}`,
      target: "_blank",
      "aria-label": address,
    },
  }, $api.i(lines, (line, index) => $api.h("div", {
    key: index + 1,
    className: "slds-truncate",
  }, [$api.t(line)])))];
}

export default createBaseComponent("lightning-formatted-address", renderFormattedAddress);
