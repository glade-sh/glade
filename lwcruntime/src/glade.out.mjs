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
const modules = (config.manifest && config.manifest.modules) || {};

function normalizeQualified(name) {
  return String(name || "").trim().toLowerCase();
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

export async function mountComponent(qualified, attrs, locator) {
  const key = normalizeQualified(qualified);
  const entry = modules[key];
  if (!entry) {
    console.warn("[glade] unknown LWC component", qualified);
    return null;
  }
  const mod = await import(entry.url);
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
      console.warn("[glade] Lightning Out app not indexed", app);
    }
    callback();
  },
  createComponent(qualified, attrs, locator, callback) {
    mountComponent(qualified, attrs, locator)
      .then((el) => {
        if (typeof callback !== "function") {
          return;
        }
        if (el) {
          callback(el, "SUCCESS");
          return;
        }
        callback(null, "ERROR", "Failed to create component " + qualified);
      })
      .catch((err) => {
        console.error("[glade] createComponent failed", err);
        if (typeof callback === "function") {
          callback(null, "ERROR", String(err));
        }
      });
  },
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
