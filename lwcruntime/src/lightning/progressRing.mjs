import { createBaseComponent } from "./base.mjs";

function renderProgressRing($api, $cmp) {
  const { h } = $api;
  const number = Number($cmp.value);
  const progress = Number.isFinite(number) ? Math.min(100, Math.max(0, number)) : 0;
  const angle = progress * 2 * Math.PI / 100;
  const x = Math.cos(angle);
  const y = -Math.sin(angle);
  const path = progress === 100
    ? "M1 0 A1 1 0 1 0 -1 0 A1 1 0 1 0 1 0 Z"
    : `M0 0 L1 0 A1 1 0 ${progress > 50 ? 1 : 0} 0 ${x} ${y} Z`;
  const head = number > 0 && number < 100 ? [h("circle", {
    key: 7,
    svg: true,
    className: "slds-progress-ring__path",
    attrs: { cx: x * 0.7, cy: y * 0.7, r: "0.1" },
  }, [])] : [];
  return [h("div", { key: 0, className: "slds-progress-ring" }, [
    h("div", {
      key: 1,
      className: "slds-progress-ring__progress",
      attrs: {
        role: "progressbar",
        "aria-label": "Progress Ring",
        "aria-valuemin": "0",
        "aria-valuemax": "100",
        "aria-valuenow": $cmp.value,
      },
    }, [h("svg", { key: 2, svg: true, attrs: { viewBox: "-1 -1 2 2" } }, [
      h("path", { key: 3, svg: true, className: "slds-progress-ring__path", attrs: { d: path } }, []),
    ])]),
    h("div", { key: 4, className: "slds-progress-ring__content" }, []),
    h("div", { key: 5, className: "slds-progress-ring__progress-head" }, [
      h("svg", { key: 6, svg: true, attrs: { viewBox: "-1 -1 2 2" } }, head),
    ]),
  ])];
}

export default createBaseComponent("lightning-progress-ring", renderProgressRing);
