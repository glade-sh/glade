const diagnosticCode = "GLADELWC092";

const localState = {
  initialized: false,
  enclosingUtilityId: "",
  utilities: [],
  callbacks: new Map(),
};

function context() {
  if (typeof document === "undefined") {
    return {};
  }
  const node = document.getElementById("glade-lwc-context");
  if (!node) {
    return {};
  }
  try {
    return JSON.parse(node.textContent || "{}");
  } catch (_err) {
    return {};
  }
}

function currentUrl() {
  return typeof window === "undefined" ? "" : window.location?.href || "";
}

function reportDiagnostic(message) {
  if (typeof window === "undefined") {
    return null;
  }
  const diagnostics = window.__gladeDiagnostics || [];
  window.__gladeDiagnostics = diagnostics;
  const entry = {
    code: diagnosticCode,
    severity: "info",
    message,
  };
  diagnostics.push(entry);
  if (typeof document !== "undefined" && typeof document.dispatchEvent === "function" && typeof CustomEvent === "function") {
    document.dispatchEvent(new CustomEvent("glade:diagnostic", { detail: entry }));
  }
  return entry;
}

function normalizeUtility(item = {}, index = 0) {
  const id = String(item.id || item.utilityId || item.componentName || `utility-${index + 1}`);
  const label = String(item.label || item.title || id || "Utility");
  return {
    id,
    utilityId: id,
    label,
    title: String(item.title || label),
    componentName: String(item.componentName || ""),
    url: String(item.url || currentUrl()),
    icon: String(item.icon || "utility:preview"),
    iconVariant: item.iconVariant || null,
    highlighted: Boolean(item.highlighted),
    panelHeaderLabel: String(item.panelHeaderLabel || label),
    panelHeaderIcon: String(item.panelHeaderIcon || item.icon || "utility:preview"),
    panelVisible: Boolean(item.panelVisible || item.open),
    minimized: !Boolean(item.panelVisible || item.open),
    focused: Boolean(item.focused),
    modalMode: Boolean(item.modalMode),
    popoutEnabled: item.popoutEnabled !== false,
    poppedOut: Boolean(item.poppedOut),
    disabled: Boolean(item.disabled),
    disabledText: String(item.disabledText || ""),
    height: Number(item.height || item.heightPX || 340),
    width: Number(item.width || item.widthPX || 480),
    gladeDiagnostic: diagnosticCode,
  };
}

function ensureState() {
  if (localState.initialized) {
    return localState;
  }
  const shell = context();
  const utilities = Array.isArray(shell.workspace?.utilities) ? shell.workspace.utilities : [];
  localState.utilities = utilities.map(normalizeUtility);
  if (!localState.utilities.length) {
    localState.utilities = [normalizeUtility({ id: "local-utility", label: "Local Utility" })];
  }
  localState.enclosingUtilityId = String(shell.workspace?.enclosingUtilityId || localState.utilities[0]?.id || "");
  localState.initialized = true;
  reportDiagnostic("Using simulated local lightning/platformUtilityBarApi state.");
  return localState;
}

function cloneUtility(item) {
  return item ? { ...item } : null;
}

function readEnclosingUtilityId() {
  const state = ensureState();
  return state.enclosingUtilityId || null;
}

function utilityIdFrom(value) {
  if (typeof value === "string") {
    return value;
  }
  return value?.utilityId || value?.id || readEnclosingUtilityId();
}

function findUtility(value) {
  const state = ensureState();
  const id = utilityIdFrom(value);
  let utility = state.utilities.find((item) => item.id === id || item.utilityId === id);
  if (!utility) {
    utility = normalizeUtility({ id: id || "local-utility", label: id || "Local Utility" }, state.utilities.length);
    state.utilities.push(utility);
  }
  return utility;
}

function notifyUtilityClick(utility) {
  const callbacks = ensureState().callbacks.get(utility.id);
  if (!callbacks) {
    return;
  }
  for (const callback of callbacks) {
    callback(cloneUtility(utility));
  }
}

function updateUtilityFields(utility, attrs = {}) {
  if (attrs.label !== undefined) {
    utility.label = String(attrs.label);
  }
  if (attrs.icon !== undefined) {
    utility.icon = String(attrs.icon);
  }
  if (attrs.iconVariant !== undefined) {
    utility.iconVariant = attrs.iconVariant;
  }
  if (attrs.highlighted !== undefined) {
    utility.highlighted = Boolean(attrs.highlighted);
  }
}

function updatePanelFields(utility, attrs = {}) {
  if (attrs.label !== undefined) {
    utility.panelHeaderLabel = String(attrs.label);
  }
  if (attrs.icon !== undefined) {
    utility.panelHeaderIcon = String(attrs.icon);
  }
  if (attrs.height !== undefined || attrs.heightPX !== undefined) {
    utility.height = Number(attrs.height ?? attrs.heightPX);
  }
  if (attrs.width !== undefined || attrs.widthPX !== undefined) {
    utility.width = Number(attrs.width ?? attrs.widthPX);
  }
  updateUtilityFields(utility, attrs);
}

export function EnclosingUtilityId(dataCallback) {
  if (typeof dataCallback === "function") {
    this.dataCallback = dataCallback;
    return;
  }
  return Promise.resolve(readEnclosingUtilityId());
}

EnclosingUtilityId.prototype.connect = function connect() {
  if (typeof this.dataCallback === "function") {
    this.dataCallback(readEnclosingUtilityId());
  }
};
EnclosingUtilityId.prototype.disconnect = function disconnect() {};
EnclosingUtilityId.prototype.update = EnclosingUtilityId.prototype.connect;

export function getAllUtilityInfo() {
  return Promise.resolve(ensureState().utilities.map(cloneUtility));
}

export function getInfo(utilityId) {
  return Promise.resolve(cloneUtility(findUtility(utilityId)));
}

export function getUtilityInfo(configOrId = {}) {
  return getInfo(utilityIdFrom(configOrId));
}

export function open(utilityId, options = {}) {
  const utility = findUtility(utilityId);
  utility.panelVisible = true;
  utility.minimized = false;
  if (options?.autoFocus) {
    utility.focused = true;
  }
  notifyUtilityClick(utility);
  return Promise.resolve(true);
}

export function openUtility(config = {}) {
  return open(utilityIdFrom(config), config);
}

export function closeUtility(config = {}) {
  const utility = findUtility(config);
  utility.panelVisible = false;
  utility.minimized = true;
  utility.focused = false;
  return Promise.resolve(true);
}

export function minimize(utilityId) {
  return closeUtility({ utilityId });
}

export function minimizeUtility(config = {}) {
  return closeUtility(config);
}

export function focusUtility(configOrId = {}) {
  const state = ensureState();
  const utility = findUtility(configOrId);
  for (const item of state.utilities) {
    item.focused = item.id === utility.id;
  }
  return Promise.resolve(true);
}

export function onUtilityClick(utilityId, eventHandler) {
  const id = utilityIdFrom(utilityId);
  const handler = typeof utilityId === "object" && typeof utilityId?.eventHandler === "function"
    ? utilityId.eventHandler
    : eventHandler;
  if (typeof handler !== "function") {
    return Promise.resolve(true);
  }
  const utility = findUtility(id);
  const callbacks = ensureState().callbacks.get(utility.id) || new Set();
  callbacks.add(handler);
  ensureState().callbacks.set(utility.id, callbacks);
  return Promise.resolve(true);
}

export function updateUtility(utilityId, attrs = {}) {
  updateUtilityFields(findUtility(utilityId), attrs);
  return Promise.resolve(true);
}

export function updatePanel(utilityId, attrs = {}) {
  updatePanelFields(findUtility(utilityId), attrs);
  return Promise.resolve(true);
}

export function setUtilityLabel(config = {}) {
  return updateUtility(utilityIdFrom(config), { label: config.label });
}

export function setUtilityIcon(config = {}) {
  return updateUtility(utilityIdFrom(config), { icon: config.icon });
}

export function setUtilityHighlighted(config = {}) {
  return updateUtility(utilityIdFrom(config), { highlighted: config.highlighted });
}

export function setPanelHeaderLabel(config = {}) {
  return updatePanel(utilityIdFrom(config), { label: config.label });
}

export function setPanelHeaderIcon(config = {}) {
  return updatePanel(utilityIdFrom(config), { icon: config.icon });
}

export function setPanelHeight(config = {}) {
  return updatePanel(utilityIdFrom(config), { heightPX: config.heightPX });
}

export function setPanelWidth(config = {}) {
  return updatePanel(utilityIdFrom(config), { widthPX: config.widthPX });
}

export function enableModal(utilityId, enabled) {
  findUtility(utilityId).modalMode = Boolean(enabled);
  return Promise.resolve(true);
}

export function toggleModalMode(config = {}) {
  return enableModal(utilityIdFrom(config), config.enableModalMode);
}

export function enablePopout(utilityId, enabled, options = {}) {
  const utility = findUtility(utilityId);
  utility.popoutEnabled = Boolean(enabled);
  utility.disabledText = String(options.disabledText || "");
  return Promise.resolve(true);
}

export function disableUtilityPopOut(config = {}) {
  return enablePopout(utilityIdFrom(config), !config.disabled, { disabledText: config.disabledText });
}

export function isUtilityPoppedOut(utilityId) {
  return Promise.resolve(Boolean(findUtility(utilityId).poppedOut));
}

export default {
  EnclosingUtilityId,
  closeUtility,
  disableUtilityPopOut,
  enableModal,
  enablePopout,
  focusUtility,
  getAllUtilityInfo,
  getInfo,
  getUtilityInfo,
  isUtilityPoppedOut,
  minimize,
  minimizeUtility,
  onUtilityClick,
  open,
  openUtility,
  setPanelHeaderIcon,
  setPanelHeaderLabel,
  setPanelHeight,
  setPanelWidth,
  setUtilityHighlighted,
  setUtilityIcon,
  setUtilityLabel,
  toggleModalMode,
  updatePanel,
  updateUtility,
};
