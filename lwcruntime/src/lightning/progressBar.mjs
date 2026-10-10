import { createBaseComponent } from "./base.mjs";

function renderProgressBar($api, $cmp) {
  const number = Number($cmp.value);
  const value = Number.isFinite(number) ? Math.min(100, Math.max(0, number)) : 0;
  const { h, t } = $api;
  return [h("div", {
    key: 0,
    className: `slds-progress-bar slds-progress-bar_${$cmp.size || "medium"}`,
    attrs: {
      role: "progressbar",
      "aria-label": "Progress Bar",
      "aria-valuemin": "0",
      "aria-valuemax": "100",
      "aria-valuenow": String(value),
    },
  }, [h("span", {
    key: 1,
    className: "slds-progress-bar__value",
    style: `width: ${value}%`,
  }, [h("span", { key: 2, className: "slds-assistive-text" }, [
    t(`Progress ${value}%`),
  ])])])];
}

export default createBaseComponent("lightning-progress-bar", renderProgressBar);
