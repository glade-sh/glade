export function readCommunityShell() {
  const node = document.getElementById("glade-lwc-context");
  if (!node) {
    return {};
  }
  try {
    return JSON.parse(node.textContent || "{}").community || {};
  } catch (_err) {
    return {};
  }
}

export function readRouteParam(name, fallback = "") {
  const value = readCommunityShell().routeParams?.[name];
  return value === undefined || value === null || value === "" ? fallback : String(value);
}

export function readManagedContent(key) {
  return readCommunityShell().managedContent?.[key] || null;
}

export function readMenu(name = "main") {
  return readCommunityShell().menus?.[name] || [];
}
