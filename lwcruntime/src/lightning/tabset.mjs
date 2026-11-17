import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import ButtonMenu from "./buttonMenu.mjs";
import MenuItem from "./menuItem.mjs";

// Tab bar fit margin in pixels. Native API 67.0 and 68.0 captures at 8 px steps
// in a 12-tab set (tab widths 112.81, 133.38, 118.03, 127.22, 113.64 px, More
// button 82.41 px) put each step to 2, 3, 4, 5 and 11 visible tabs, and the
// all-visible step, between the width of those tabs plus the More button plus a
// margin of 27.7 to 31.4 px (the all-visible step has no More button). 29.5 is
// the middle of that range. It is a fitted constant, not a documented value, and
// it was fitted with the org's own text widths: the rule scales with the widths
// the browser measures.
const TAB_FIT_MARGIN = 29.5;

// The indexes of the tabs that stay in the bar. Fit rule, fitted to the native
// API 67.0 and 68.0 captures (see TAB_FIT_MARGIN): the leading n tabs stay when
// their widths, the margin, and the More button (unless every tab fits) fit the
// container; at least one stays, and a measured fit that holds every tab leaves
// the menu empty. A selected tab that is not among the leading tabs stays in
// the bar too, and the leading tabs that fit beside it keep their places: the
// captures select tab 5 of 6 at 480 px and get tabs 1, 2 and 5, tab 4 and get
// tabs 1 and 4, and at 260 px any selected tab is alone in the bar.
function fitTabs(widths, overflow, width, active) {
  const count = widths.length;
  const fits = (sum, n) => sum + TAB_FIT_MARGIN + (n < count ? overflow : 0) <= width;
  let total = 0;
  let leading = 0;
  for (let n = 1; n <= count; n++) {
    total += widths[n - 1];
    if (fits(total, n)) leading = n;
  }
  leading = Math.max(leading, 1);
  if (active < 0 || active < leading) return Array.from({ length: leading }, (_, index) => index);
  let kept = 0;
  let sum = widths[active];
  for (let k = 1; k < active; k++) {
    sum += widths[k - 1];
    if (!fits(sum, k + 1)) break;
    kept = k;
  }
  return [...Array.from({ length: kept }, (_, index) => index), active];
}

function renderTabBar($api, $cmp) {
  const { h, t, c, b, i } = $api;
  const allHeaders = $cmp.headers || [];
  // Native API 67/68 captures (tabsets of 3 to 12 tabs in containers of 260 to
  // 2400 px): tabs that do not fit leave the tab bar and are listed only in the
  // "More Tabs" menu. With no measured layout every header keeps both places.
  const headers = $cmp.visibleKeys == null ? allHeaders : allHeaders.filter(header => $cmp.visibleKeys.includes(header.key));
  const menuHeaders = $cmp.visibleKeys == null ? allHeaders : allHeaders.filter(header => !$cmp.visibleKeys.includes(header.key));
  const items = i(headers, header => {
    const label = header.label == null ? "" : String(header.label);
    return h("li", {
      key: "tab-" + header.key,
      className: "slds-tabs_default__item" + (header.selected ? " slds-is-active" : "")
        + (header.selected && $cmp.focusedKey === header.key ? " slds-has-focus" : ""),
      attrs: { role: "presentation", title: header.label == null ? null : label },
    }, [h("a", {
      key: 0,
      className: "slds-tabs_default__link",
      attrs: {
        href: "javascript:void(0)",
        role: "tab",
        tabindex: header.selected ? "0" : "-1",
        "aria-selected": String(header.selected),
        "data-tab-key": header.key,
      },
      on: { click: b($cmp.handleSelect), focus: b($cmp.handleFocus) },
    }, [t(label)])]);
  });
  const overflow = h("li", {
    key: "overflow",
    className: "slds-tabs_default__item slds-tabs_default__overflow-button",
    attrs: { role: "presentation" },
  }, [c("lightning-button-menu", ButtonMenu, {
    key: 1,
    props: { label: "More", alternativeText: "Tabs", title: "More Tabs", variant: "bare" },
    on: { select: b($cmp.handleMenuSelect) },
  }, i(menuHeaders, header => c("lightning-menu-item", MenuItem, {
    key: "menu-" + header.key,
    props: { label: header.label, value: header.key },
  }, [])))]);
  return [h("ul", {
    key: 0,
    className: "slds-tabs_default__nav",
    attrs: { role: "tablist", "aria-label": "Tabs" },
  }, $api.f([items, [overflow]]))];
}
renderTabBar.stylesheets = [];
const barTemplate = registerTemplate(renderTabBar);
freezeTemplate(barTemplate);

class TabBar extends LightningElement {
  get headers() {
    return this.headersValue;
  }

  set headers(value) {
    this.headersValue = value;
    this.visibleKeys = undefined;
  }

  connectedCallback() {
    if (typeof ResizeObserver !== "function") return;
    this.resizeObserver = new ResizeObserver(() => {
      const width = this.containerWidth();
      if (width !== this.lastWidth && this.visibleKeys !== undefined) this.visibleKeys = undefined;
    });
    this.resizeObserver.observe(this.template.host.parentElement || this.template.host);
  }

  disconnectedCallback() {
    this.resizeObserver?.disconnect();
    this.resizeObserver = undefined;
  }

  containerWidth() {
    return (this.template.host.parentElement || this.template.host).getBoundingClientRect().width;
  }

  renderedCallback() {
    if (this.visibleKeys !== undefined) return;
    const ul = this.template.querySelector("ul");
    const width = this.containerWidth();
    this.lastWidth = width;
    if (!ul || !(width > 0)) return;
    // Measure natural widths: a max-content list does not shrink its items.
    const previous = ul.style.width;
    ul.style.width = "max-content";
    const widths = [...ul.children].map(item => item.getBoundingClientRect().width);
    ul.style.width = previous;
    const overflow = widths.pop() || 0;
    if (!widths.length) return;
    this.visibleKeys = fitTabs(widths, overflow, width, (this.headers || []).findIndex(header => header.selected)).map(index => this.headers[index].key);
  }

  handleSelect(event) {
    event.preventDefault();
    const key = event.currentTarget.dataset.tabKey;
    const header = this.headers?.find(header => header.key === key);
    if (header && !header.selected) {
      this.focusedKey = key;
      event.currentTarget.focus();
    }
    this.selectKey(key);
  }

  handleFocus(event) {
    this.focusedKey = event.currentTarget.dataset.tabKey;
  }

  handleMenuSelect(event) {
    this.selectKey(event.detail.value);
  }

  selectKey(key) {
    this.dispatchEvent(new CustomEvent("privatetabselect", {
      bubbles: true,
      composed: true,
      detail: { key },
    }));
  }
}
registerDecorators(TabBar, {
  publicProps: { headers: { config: 3 } },
  fields: ["focusedKey", "headersValue", "visibleKeys"],
});
const LightningTabBar = registerComponent(TabBar, {
  tmpl: barTemplate,
  sel: "lightning-tab-bar",
  apiVersion: 63,
});

function renderTabset($api, $cmp, $slotset) {
  return [$api.h("div", { key: 0, classMap: { "slds-tabs_default": true } }, [
    $api.c("lightning-tab-bar", LightningTabBar, { key: 1, props: { headers: $cmp.headers } }, []),
    $api.s("", { key: 2 }, [], $slotset),
  ])];
}
renderTabset.stylesheets = [];
renderTabset.slots = [""];
const template = registerTemplate(renderTabset);
freezeTemplate(renderTabset);

class Tabset extends LightningElement {
  constructor() {
    super();
    this.tabs = new Map();
    this.nextKey = 0;
    this.headers = [];
    this.selectedTab = null;
    this.pendingValue = undefined;
    this.headersPending = false;
    this.registerTab = this.registerTab.bind(this);
    this.activateTab = this.activateTab.bind(this);
  }

  connectedCallback() {
    this.addEventListener("privatetabregister", this.registerTab);
    this.addEventListener("privatetabselect", this.activateTab);
  }

  disconnectedCallback() {
    this.removeEventListener("privatetabregister", this.registerTab);
    this.removeEventListener("privatetabselect", this.activateTab);
    this.tabs.clear();
    this.selectedTab = null;
  }

  get activeTabValue() {
    return this.selectedTab?.getValue();
  }

  set activeTabValue(value) {
    if (typeof value !== "string" || !value) return;
    const tab = [...this.tabs.values()].find(tab => tab.getValue() === value);
    if (tab) {
      this.selectTab(tab, true);
    } else if (!this.tabs.size) {
      this.pendingValue = value;
    }
  }

  registerTab(event) {
    event.stopPropagation();
    const tab = event.detail;
    if (!tab || typeof tab.getValue !== "function" || typeof tab.setSelected !== "function") return;
    const key = String(this.nextKey++);
    this.tabs.set(key, tab);
    tab.update = () => this.updateHeaders();
    // Native selected/inactive DOM-removal controls retain the registered
    // headers and selection; do not invent per-tab disconnect cleanup.
    const requested = typeof this.pendingValue === "string" && tab.getValue() === this.pendingValue;
    if (!this.selectedTab || requested) {
      this.selectTab(tab, false);
      if (requested) this.pendingValue = undefined;
    } else {
      tab.setSelected(false);
    }
    this.updateHeaders();
  }

  activateTab(event) {
    event.stopPropagation();
    const tab = this.tabs.get(event.detail.key);
    if (tab) this.selectTab(tab, true);
  }

  selectTab(selected, notify) {
    if (this.selectedTab === selected) return;
    this.selectedTab = selected;
    for (const tab of this.tabs.values()) tab.setSelected(tab === selected);
    this.updateHeaders();
    if (notify) selected.activate();
  }

  updateHeaders() {
    if (this.headersPending) return;
    this.headersPending = true;
    Promise.resolve().then(() => {
      this.headersPending = false;
      if (!this.isConnected) return;
      this.headers = [...this.tabs].map(([key, tab]) => ({
        key,
        label: tab.getLabel(),
        selected: tab === this.selectedTab,
      }));
    });
  }
}

registerDecorators(Tabset, {
  publicProps: { activeTabValue: { config: 3 }, variant: { config: 0 } },
  fields: ["headers"],
});
export default registerComponent(Tabset, {
  tmpl: template,
  sel: "lightning-tabset",
  apiVersion: 63,
});
