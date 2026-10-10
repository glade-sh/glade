import {
  LightningElement,
  freezeTemplate,
  registerComponent,
  registerDecorators,
  registerTemplate,
} from "lwc";
import ButtonMenu from "./buttonMenu.mjs";
import MenuItem from "./menuItem.mjs";

function renderTabBar($api, $cmp) {
  const { h, t, c, b, i } = $api;
  const headers = $cmp.headers || [];
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
    attrs: { role: "tab" },
  }, [c("lightning-button-menu", ButtonMenu, {
    key: 1,
    props: { label: "More", alternativeText: "Tabs", title: "More Tabs", variant: "bare" },
    on: { select: b($cmp.handleMenuSelect) },
  }, i(headers, header => c("lightning-menu-item", MenuItem, {
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
  publicProps: { headers: { config: 0 } },
  fields: ["focusedKey"],
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
