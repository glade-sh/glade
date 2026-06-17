import "@lwc/synthetic-shadow";
import { createElement } from "lwc";
function readConfig() {
  const node = document.getElementById("glade-lightning-config");
  if (!node) {
    return { manifest: { modules: {} }, namespace: "c" };
  }
  try {
    return JSON.parse(node.textContent || "{}");
  } catch (_err) {
    return { manifest: { modules: {} }, namespace: "c" };
  }
}
const config = readConfig();
const modules = config.manifest && config.manifest.modules || {};
let activeOutApp = "";
function normalizeQualified(name) {
  return String(name || "").trim().toLowerCase();
}
function hasNamespace(name) {
  return normalizeQualified(name).includes(":");
}
function isLightningService(name) {
  return normalizeQualified(name).startsWith("lightning:");
}
function lightningOutDiagnostic(code, message, value) {
  return `${code} ${message}: ${value}`;
}
function outAppDependencies(app) {
  const deps = config.outAppDependencies || {};
  const values = deps[normalizeQualified(app)];
  if (!Array.isArray(values)) {
    return null;
  }
  return values.map(normalizeQualified);
}
function validateComponentRequest(qualified) {
  if (!hasNamespace(qualified)) {
    return "Bad Lightning component name: " + qualified;
  }
  if (isLightningService(qualified)) {
    return lightningOutDiagnostic("GLADELWC082", "Lightning Out service unsupported in Visualforce host", qualified);
  }
  const deps = outAppDependencies(activeOutApp);
  if (deps && !deps.includes(normalizeQualified(qualified))) {
    return lightningOutDiagnostic("GLADELWC081", "Lightning Out dependency missing", qualified);
  }
  return "";
}
function resolveHost(locator) {
  if (!locator) {
    return null;
  }
  const byId = document.getElementById(locator);
  if (byId) {
    return byId;
  }
  try {
    return document.querySelector(locator);
  } catch (_err) {
    return null;
  }
}
function applyPublicProperties(el, attrs) {
  if (!attrs || typeof attrs !== "object") {
    return;
  }
  for (const [name, value] of Object.entries(attrs)) {
    el[name] = value;
  }
}
async function mountComponent(qualified, attrs, locator) {
  const diagnostic = validateComponentRequest(qualified);
  if (diagnostic) {
    throw new Error(diagnostic);
  }
  const key = normalizeQualified(qualified);
  const entry = modules[key];
  if (!entry) {
    console.warn("[glade] unknown LWC component", qualified);
    return null;
  }
  let mod;
  try {
    mod = await import(entry.url);
  } catch (err) {
    throw new Error(lightningOutDiagnostic("GLADELWC081", "Lightning Out dependency missing", qualified), { cause: err });
  }
  const Ctor = mod.default;
  if (typeof Ctor !== "function") {
    console.warn("[glade] LWC module missing default export", qualified);
    return null;
  }
  const tag = entry.tag || key.replace(":", "-");
  const el = createElement(tag, { is: Ctor });
  applyPublicProperties(el, attrs);
  const host = resolveHost(locator);
  if (!host) {
    console.warn("[glade] missing mount locator", locator);
    return null;
  }
  host.replaceChildren(el);
  return el;
}
window.$Lightning = {
  use(app, callback) {
    if (typeof callback !== "function") {
      return;
    }
    const outApp = normalizeQualified(app);
    const allowed = (config.outApps || []).map(normalizeQualified);
    if (allowed.length > 0 && !allowed.includes(outApp)) {
      console.error("[glade] Lightning Out app not found", app);
      callback(null, "ERROR", lightningOutDiagnostic("GLADELWC080", "Lightning Out app missing", app));
      return;
    }
    activeOutApp = outApp;
    callback();
  },
  createComponent(qualified, attrs, locator, callback) {
    mountComponent(qualified, attrs, locator).then((el) => {
      if (typeof callback !== "function") {
        return;
      }
      if (el) {
        callback(el, "SUCCESS");
        return;
      }
      callback(null, "ERROR", lightningOutDiagnostic("GLADELWC081", "Lightning Out dependency missing", qualified));
    }).catch((err) => {
      console.error("[glade] createComponent failed", err);
      if (typeof callback === "function") {
        callback(null, "ERROR", err && err.message ? err.message : String(err));
      }
    });
  }
};
const pending = window.__gladeLightningPending || [];
delete window.__gladeLightningPending;
for (const item of pending) {
  if (!Array.isArray(item) || item.length === 0) {
    continue;
  }
  if (item[0] === "use") {
    window.$Lightning.use(item[1], item[2]);
  } else if (item[0] === "create") {
    window.$Lightning.createComponent(item[1], item[2], item[3], item[4]);
  }
}
export {
  mountComponent
};
