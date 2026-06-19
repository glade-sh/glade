function readJSON(id) {
  if (typeof document === "undefined") {
    return {};
  }
  const node = document.getElementById(id);
  if (!node) {
    return {};
  }
  try {
    return JSON.parse(node.textContent || "{}");
  } catch (_err) {
    return {};
  }
}

function workbench() {
  return readJSON("glade-lwc-workbench");
}

function config() {
  return readJSON("glade-lightning-config");
}

function routeNavType(route = {}) {
  switch (route.kind) {
    case "record":
    case "recordPage":
    case "standard__recordPage":
      return "standard__recordPage";
    case "appPage":
    case "homePage":
    case "standard__navItemPage":
      return "standard__navItemPage";
    default:
      return route.kind || "standard__component";
  }
}

function navItemName(route = {}) {
  return route.tabName || route.pageName || route.objectApiName || route.component || route.label || route.url || "";
}

function navItemForRoute(route = {}) {
  const name = navItemName(route);
  return {
    apiName: name,
    availableInClassic: false,
    color: null,
    content: null,
    custom: !String(name).startsWith("standard-") && !String(name).startsWith("standard__"),
    developerName: name,
    iconUrl: null,
    id: name,
    itemType: routeNavType(route),
    label: route.label || name,
    objectApiName: route.objectApiName || null,
    pageReference: route.pageReference || null,
    url: route.url || null,
  };
}

function navItemsFromWorkbench(model = workbench()) {
  const routes = Array.isArray(model.routes) ? model.routes : [];
  const app = Array.isArray(model.apps) ? model.apps[0] : null;
  const selected = new Set((app?.navItems || []).map((item) => String(item).toLowerCase()));
  return routes
    .filter((route) => {
      if (!selected.size) {
        return route.kind === "tab" || route.tabName;
      }
      const names = [route.tabName, route.pageName, route.objectApiName, route.label].filter(Boolean);
      return names.some((name) => selected.has(String(name).toLowerCase()));
    })
    .map(navItemForRoute);
}

function menuItemsFromWorkbench(model = workbench()) {
  const apps = Array.isArray(model.apps) ? model.apps : [];
  if (apps.length) {
    return apps.map((app) => ({
      applicationId: app.name || app.label || "Local",
      label: app.label || app.name || "Local",
      name: app.name || app.label || "Local",
      type: app.mode === "console" ? "Console" : "Standard",
      url: app.defaultUrl || "/lwc",
    }));
  }
  const shellConfig = config();
  const modules = shellConfig?.manifest?.modules || {};
  const names = Object.keys(modules);
  if (!names.length) {
    return [];
  }
  return [{
    applicationId: "Local",
    label: "Local",
    name: "Local",
    type: "Standard",
    url: "/lwc",
  }];
}

function navItemsPayload() {
  return { data: { navItems: navItemsFromWorkbench() }, error: undefined };
}

export function getNavItems(configOrCallback) {
  if (typeof configOrCallback === "function") {
    this.dataCallback = configOrCallback;
    return;
  }
  void configOrCallback;
  return Promise.resolve({ navItems: navItemsFromWorkbench() });
}

getNavItems.prototype.connect = function connect() {
  if (typeof this.dataCallback === "function") {
    this.dataCallback(navItemsPayload());
  }
};
getNavItems.prototype.disconnect = function disconnect() {};
getNavItems.prototype.update = getNavItems.prototype.connect;

export function getAppMenuItems() {
  return Promise.resolve({ appMenuItems: menuItemsFromWorkbench() });
}

export function getAppMenuItem(configOrId = {}) {
  const id = typeof configOrId === "string"
    ? configOrId
    : configOrId.appMenuItemId || configOrId.applicationId || configOrId.name;
  const key = String(id || "").toLowerCase();
  const appMenuItem = menuItemsFromWorkbench().find((item) => {
    return [item.applicationId, item.name, item.label].some((value) => String(value || "").toLowerCase() === key);
  }) || null;
  return Promise.resolve({ appMenuItem });
}
