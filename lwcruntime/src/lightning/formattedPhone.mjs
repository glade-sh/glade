import { createBaseComponent } from "./base.mjs";

function renderFormattedPhone($api, $cmp) {
  const value = $cmp.value;
  if (value == null || value === "" || typeof value === "number") return [];
  return [$api.h("a", {
    key: 0,
    attrs: { href: `tel:${value || ""}`, target: $cmp.target || undefined },
  }, [$api.t($cmp.label || value || $cmp.href || "")])];
}

export default createBaseComponent("lightning-formatted-phone", renderFormattedPhone);
