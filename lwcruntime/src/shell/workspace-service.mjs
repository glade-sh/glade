const state = {
  console: false,
  focusedTabId: "workspace-tab-1",
  tabs: [{ tabId: "workspace-tab-1", label: "Local", url: currentUrl(), workspaceTab: true }],
  utilities: [],
};

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

export async function getFocusedTabInfo() {
  return cloneTab(state.tabs.find((tab) => tab.tabId === state.focusedTabId) || state.tabs[0]);
}

export async function getAllTabInfo() {
  return state.tabs.map(cloneTab);
}

export async function getTabInfo(tabId) {
  return cloneTab(state.tabs.find((tab) => tab.tabId === tabId) || state.tabs[0]);
}

export async function openTab(options = {}) {
  const tab = normalizeTab({
    tabId: options.tabId || `workspace-tab-${state.tabs.length + 1}`,
    label: options.label || options.title || options.url || "Tab",
    url: options.url || "",
    workspaceTab: true,
  });
  state.tabs.push(tab);
  state.focusedTabId = tab.tabId;
  return tab.tabId;
}

export async function openSubtab(_parentTabId, options = {}) {
  return openTab({ ...options, workspaceTab: false });
}

export async function closeTab(tabId) {
  const index = state.tabs.findIndex((tab) => tab.tabId === tabId);
  if (index >= 0) {
    state.tabs.splice(index, 1);
  }
  if (!state.tabs.length) {
    state.tabs.push(normalizeTab({ tabId: "workspace-tab-1", label: "Local", url: currentUrl(), workspaceTab: true }));
  }
  if (state.focusedTabId === tabId) {
    state.focusedTabId = state.tabs[0].tabId;
  }
  return true;
}

export async function focusTab(tabId) {
  if (state.tabs.some((tab) => tab.tabId === tabId)) {
    state.focusedTabId = tabId;
  }
  return true;
}

export async function refreshTab(_tabId, _options = {}) {
  return true;
}

export async function disableTabClose(_tabId, _disabled) {
  return true;
}

export async function setTabLabel(tabId, label) {
  const tab = state.tabs.find((item) => item.tabId === tabId) || state.tabs[0];
  if (tab && label) {
    tab.label = String(label);
    tab.title = tab.label;
    tab.customTitle = tab.label;
  }
  return cloneTab(tab);
}

export async function setTabIcon(tabId, icon) {
  const tab = state.tabs.find((item) => item.tabId === tabId) || state.tabs[0];
  if (tab && icon) {
    tab.icon = String(icon);
  }
  return cloneTab(tab);
}

export async function setTabHighlighted(tabId, highlighted) {
  const tab = state.tabs.find((item) => item.tabId === tabId) || state.tabs[0];
  if (tab) {
    tab.highlighted = Boolean(highlighted);
  }
  return cloneTab(tab);
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

function normalizeTab(tab = {}) {
  const label = String(tab.label || tab.title || tab.url || "Local");
  return {
    tabId: String(tab.tabId || `workspace-tab-${state.tabs.length + 1}`),
    label,
    title: String(tab.title || label),
    customTitle: String(tab.customTitle || label),
    url: String(tab.url || ""),
    icon: String(tab.icon || "utility:preview"),
    highlighted: Boolean(tab.highlighted),
    closeable: Boolean(tab.closeable),
    workspaceTab: tab.workspaceTab !== false,
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

function cloneTab(tab) {
  return tab ? { ...tab } : null;
}

function currentUrl() {
  return typeof window === "undefined" ? "" : window.location.href;
}
