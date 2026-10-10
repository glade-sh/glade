import { createBaseComponent } from "./base.mjs";

function renderFormattedText($api, $cmp) {
  if (typeof $cmp.value !== "string" || $cmp.value === "") return [];
  return $api.f($cmp.value.split(/\r\n|\r|\n/).map((line, index) => {
    const text = $api.t(line);
    return index === 0 ? [text] : [
      $api.h("br", { key: index }, []),
      text,
    ];
  }));
}

export default createBaseComponent("lightning-formatted-text", renderFormattedText);
