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

function hostURL(value) {
  if (!value) return null;
  // The host supplies real local routes/assets. Expose absolute URLs just as
  // the native navigation API does, without embedding a deployment origin.
  if (typeof window === "undefined") return value;
  return new URL(value, window.location.href).href;
}

function navItemForRoute(route = {}) {
  const name = navItemName(route);
  return {
    availableInClassic: false,
    availableInLightning: true,
    color: route.color || null,
    content: hostURL(route.content),
    custom: !String(name).startsWith("standard-") && !String(name).startsWith("standard__"),
    developerName: name,
    iconUrl: hostURL(route.iconUrl),
    id: null,
    itemType: route.itemType || routeNavType(route),
    label: route.label || name,
    objectApiName: route.objectApiName || null,
    objectLabel: route.objectLabel || null,
    objectLabelPlural: route.objectLabelPlural || null,
    pageReference: route.pageReference || (route.tabName
      ? { type: "standard__navItemPage", attributes: { apiName: route.tabName }, state: {} }
      : null),
    standardType: route.standardType || null,
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

function navItemsPayload(options = {}) {
  let body;
  if (typeof options.pageSize === "number" && (options.pageSize <= 0 || options.pageSize > 100)) {
    body = { errorCode: "ILLEGAL_QUERY_PARAMETER_VALUE", id: "1156057470", message: "pageSize value must be greater than 0 and less than or equal to 100", statusCode: 400 };
  } else if (typeof options.page === "number" && options.page < 0) {
    body = { errorCode: "ILLEGAL_QUERY_PARAMETER_VALUE", id: "-1083006838", message: "page value must be greater than or equal to 0", statusCode: 400 };
  }
  if (body) return { data: undefined, error: { body, errorType: "fetchResponse", headers: {}, ok: false, status: 400, statusText: "Bad Request" } };
  const names = Array.isArray(options.navItemNames) ? options.navItemNames : [];
  const items = navItemsFromWorkbench().filter(item => names.includes(item.developerName));
  const pageSize = typeof options.pageSize === "number" ? options.pageSize : 100;
  const offset = (typeof options.page === "number" ? options.page : 0) * pageSize;
  return { data: { navItems: items.slice(offset, offset + pageSize) }, error: undefined };
}

export function getNavItems(configOrCallback) {
  if (typeof configOrCallback === "function") {
    this.dataCallback = configOrCallback;
    return;
  }
  const value = navItemsPayload(configOrCallback || {});
  return value.error ? Promise.reject(value.error) : Promise.resolve(value.data);
}

getNavItems.prototype.connect = function connect() {
  if (typeof this.dataCallback === "function") {
    this.dataCallback(navItemsPayload(this.config));
  }
};
getNavItems.prototype.disconnect = function disconnect() {};
getNavItems.prototype.update = function update(options) {
  this.config = options || {};
  this.connect();
};

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
