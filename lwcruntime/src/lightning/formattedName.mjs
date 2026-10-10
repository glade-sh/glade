import { createBaseComponent } from "./base.mjs";

function renderFormattedName($api, $cmp) {
  const text = [
    $cmp.salutation,
    $cmp.firstName,
    $cmp.middleName,
    $cmp.lastName,
    $cmp.suffix,
    $cmp.informalName,
    $cmp.value,
  ].filter(Boolean).join(" ");
  return [$api.t(text)];
}

export default createBaseComponent("lightning-formatted-name", renderFormattedName);
