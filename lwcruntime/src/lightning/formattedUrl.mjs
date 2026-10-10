import { createBaseComponent } from "./base.mjs";

function renderFormattedUrl($api, $cmp) {
  if ($cmp.value == null || $cmp.value === "") return [];
  const value = String($cmp.value);
  const href = /^(?:[a-z][a-z0-9+.-]*:\/\/|\/|\.{1,2}\/)/i.test(value)
    ? value : `https://${value}`;
  return [$api.h("a", {
    key: 0,
    attrs: { href, target: $cmp.target || undefined },
  }, [$api.t($cmp.label || $cmp.value || $cmp.href || "")])];
}

export default createBaseComponent("lightning-formatted-url", renderFormattedUrl);
