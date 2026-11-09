let tabCounter = 0;
const state = {
  console: false,
  focusedTabId: "workspace-tab-1",
  tabs: [],
  utilities: [],
};
state.tabs.push(normalizeTab({ tabId: "workspace-tab-1", label: "Local", url: currentUrl(), workspaceTab: true }));

configureWorkspace(readWorkspaceContext());

export const workspaceDiagnosticCodes = ["GLADELWC072"];

export function configureWorkspace(next = {}) {
  state.console = Boolean(next.console);
  state.focusedTabId = next.focusedTabId || state.focusedTabId || "workspace-tab-1";
  if (Array.isArray(next.tabs) && next.tabs.length > 0) {
    state.tabs = next.tabs.map(normalizeTab);
  }
  if (Array.isArray(next.utilities)) {
    state.utilities = next.utilities.map(normalizeUtility);
  }
  if (!state.tabs.some((tab) => tab.tabId === state.focusedTabId)) {
    state.focusedTabId = state.tabs[0]?.tabId || "workspace-tab-1";
  }
}

// Tab behavior follows native rows captured in a scratch org Lightning console
// app (desktop Chromium, component API 59.0 and 67.0). Rejection messages,
// result shapes and option validation below are those rows. Where a row was
// not captured (openTab focus default, closing an already closed id, which
// stays pending natively) the earlier local behavior is kept.
const HIGHLIGHT_STATES = ["success", "error", "warning"];

function fail(message) {
  return Promise.reject(new Error("Error: " + message));
}

function findTab(tabId) {
  return state.tabs.find((tab) => tab.tabId === tabId);
}

function tabInfo(tab) {
  const focused = tab.tabId === state.focusedTabId || state.tabs.some((other) => other.parentTabId === tab.tabId && other.tabId === state.focusedTabId);
  const info = { ...tab, focused };
  if (!tab.isSubtab) {
    info.subtabs = state.tabs.filter((other) => other.parentTabId === tab.tabId).map(tabInfo);
  }
  return info;
}

export async function getFocusedTabInfo() {
  return tabInfo(findTab(state.focusedTabId) || state.tabs[0]);
}

export async function getAllTabInfo() {
  return state.tabs.filter((tab) => !tab.isSubtab).map(tabInfo);
}

export async function getTabInfo(tabId) {
  const tab = findTab(tabId);
  if (!tab) return fail("getTabInfo() - Failed to get tabInfo for tabId `" + tabId + "`");
  return tabInfo(tab);
}

export async function openTab(options = {}) {
  const tab = normalizeTab({
    tabId: options.tabId || `workspace-tab-${nextTabNumber()}`,
    label: options.label || options.title || options.url || "Tab",
    url: options.url || "",
    pageReference: options.pageReference,
    workspaceTab: true,
  });
  state.tabs.push(tab);
  state.focusedTabId = tab.tabId;
  return tab.tabId;
}

export async function openSubtab(parentTabId, options = {}) {
  const parent = findTab(parentTabId);
  if (!parent || parent.isSubtab) return fail("openSubtab() - Could not open subtab.");
  const tab = normalizeTab({
    tabId: options.tabId || `workspace-subtab-${nextTabNumber()}`,
    label: options.label || options.title || options.url || "Tab",
    url: options.url || "",
    pageReference: options.pageReference,
    parentTabId: parent.tabId,
    workspaceTab: false,
  });
  state.tabs.push(tab);
  // Native focus defaults to false for a subtab.
  if (options.focus === true) state.focusedTabId = tab.tabId;
  return tab.tabId;
}

export async function closeTab(tabId) {
  const tab = findTab(tabId);
  if (!tab || !tab.closeable) return fail("closeTab() - Could not close tab.");
  const closing = new Set([tab.tabId]);
  for (const other of state.tabs) if (other.parentTabId === tab.tabId) closing.add(other.tabId);
  state.tabs = state.tabs.filter((other) => !closing.has(other.tabId));
  if (!state.tabs.length) {
    state.tabs.push(normalizeTab({ tabId: "workspace-tab-1", label: "Local", url: currentUrl(), workspaceTab: true }));
  }
  if (closing.has(state.focusedTabId)) {
    state.focusedTabId = (tab.parentTabId && findTab(tab.parentTabId) ? tab.parentTabId : state.tabs[0].tabId);
  }
  return true;
}

export async function focusTab(tabId) {
  if (!findTab(tabId)) return fail("focusTab() - Could not focus tab.");
  state.focusedTabId = tabId;
  return true;
}

export async function refreshTab(tabId, _options = {}) {
  if (!findTab(tabId)) return fail("Unable to refresh tab `" + tabId + "`");
  return true;
}

export async function disableTabClose(tabId, disabled) {
  const tab = findTab(tabId);
  if (!tab) return fail("disableTabClose() - Failed to update tab disable close.");
  tab.closeable = !disabled;
  return tabInfo(tab);
}

export async function setTabLabel(tabId, label) {
  if (typeof label !== "string") return fail("Invalid or missing label `" + label + "`");
  const tab = findTab(tabId);
  if (!tab) return fail("setTabLabel() - Failed to update tab label.");
  tab.customTitle = label;
  return tabInfo(tab);
}

export async function setTabIcon(tabId, icon, options) {
  const tab = findTab(tabId);
  if (!tab) return fail("setTabIcon() - Failed to update tab icon.");
  tab.customIcon = String(icon);
  tab.customIconAlt = options?.iconAlt;
  return tabInfo(tab);
}

export async function setTabHighlighted(tabId, highlighted, options) {
  const requested = options?.state;
  if (requested !== undefined && !HIGHLIGHT_STATES.includes(requested)) {
    return fail("Invalid value `" + requested + "` for state option property");
  }
  const tab = findTab(tabId);
  if (!tab) return fail("setTabHighlighted() - Failed to update tab highlight.");
  tab.highlighted = Boolean(highlighted);
  return tabInfo(tab);
}

export async function isConsoleNavigation() {
  return state.console;
}

export function readUtilities() {
  return state.utilities.map((item) => ({ ...item }));
}

export function createValueWireAdapter(readValue) {
  function ValueWireAdapter(dataCallback) {
    this.dataCallback = dataCallback;
  }
  ValueWireAdapter.prototype.connect = function connect() {
    this.emit();
  };
  ValueWireAdapter.prototype.disconnect = function disconnect() {};
  ValueWireAdapter.prototype.update = function update() {
    this.emit();
  };
  ValueWireAdapter.prototype.emit = function emit() {
    if (typeof this.dataCallback === "function") {
      this.dataCallback(readValue());
    }
  };
  return ValueWireAdapter;
}

export const IsConsoleNavigation = createValueWireAdapter(() => state.console);
export const EnclosingTabId = createValueWireAdapter(() => state.focusedTabId);

function readWorkspaceContext() {
  const node = document.getElementById("glade-lwc-context");
  if (!node) {
    return {};
  }
  try {
    return JSON.parse(node.textContent || "{}").workspace || {};
  } catch (_err) {
    return {};
  }
}

function nextTabNumber() {
  tabCounter = Math.max(tabCounter, state.tabs.length) + 1;
  return tabCounter;
}

function normalizeTab(tab = {}) {
  const label = String(tab.label || tab.title || tab.url || "Local");
  const url = String(tab.url || "");
  const subtab = Boolean(tab.parentTabId);
  return {
    tabId: String(tab.tabId || `workspace-tab-${nextTabNumber()}`),
    label,
    title: String(tab.title || label),
    customTitle: tab.customTitle === undefined ? undefined : String(tab.customTitle),
    customIcon: tab.customIcon,
    customIconAlt: tab.customIconAlt,
    url,
    icon: String(tab.icon || "utility:preview"),
    iconAlt: tab.iconAlt === undefined ? undefined : String(tab.iconAlt),
    highlighted: Boolean(tab.highlighted),
    closeable: tab.closeable !== false,
    pinned: Boolean(tab.pinned),
    recordId: tab.recordId ?? null,
    pageReference: tab.pageReference ?? null,
    isSubtab: subtab,
    parentTabId: subtab ? String(tab.parentTabId) : null,
    workspaceTab: !subtab && tab.workspaceTab !== false,
    gladeDiagnostic: "GLADELWC072",
  };
}

function normalizeUtility(item = {}) {
  return {
    id: String(item.id || item.componentName || ""),
    label: String(item.label || item.id || item.componentName || "Utility"),
    componentName: String(item.componentName || ""),
    url: String(item.url || ""),
  };
}

function currentUrl() {
  return typeof window === "undefined" ? "" : window.location.href;
}
