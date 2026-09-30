export function readUserPermission(name) {
  return readGrantedPermission(readPermissionContext(), name);
}

export function readCustomPermission(name) {
  return readGrantedPermission(readCustomPermissionContext(), name);
}

function readGrantedPermission(permissions, name) {
  return permissions[permissionKey(name)] === true ? true : undefined;
}

function readPermissionContext() {
  const context = readShellContext();
  return normalizePermissionMap(
    context.userPermissions ||
      context.permissions?.userPermissions ||
      context.permissions ||
      {}
  );
}

function readCustomPermissionContext() {
  const context = readShellContext();
  return normalizePermissionMap(
    context.customPermissions ||
      context.permissions?.customPermissions ||
      {}
  );
}

function normalizePermissionMap(value) {
  if (Array.isArray(value)) {
    const out = {};
    for (const entry of value) {
      if (typeof entry === "string") {
        out[permissionKey(entry)] = true;
        continue;
      }
      const key = permissionKey(entry?.name || entry?.apiName || entry?.permission);
      if (key) {
        out[key] = entry?.enabled === undefined ? true : Boolean(entry.enabled);
      }
    }
    return out;
  }
  if (!value || typeof value !== "object") {
    return {};
  }
  const out = {};
  for (const [key, enabled] of Object.entries(value)) {
    out[permissionKey(key)] = Boolean(enabled);
  }
  return out;
}

function permissionKey(name) {
  return String(name || "").trim();
}

function readShellContext() {
  if (typeof document === "undefined") {
    return {};
  }
  const node = document.getElementById("glade-lwc-context");
  if (!node) {
    return {};
  }
  try {
    return JSON.parse(node.textContent || "{}") || {};
  } catch (_err) {
    return {};
  }
}
