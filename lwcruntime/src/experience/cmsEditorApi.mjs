const diagnosticCode = "GLADELWC094";

function readContext() {
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

function reportDiagnostic(message) {
  if (typeof window === "undefined") {
    return null;
  }
  const diagnostics = window.__gladeDiagnostics || [];
  window.__gladeDiagnostics = diagnostics;
  const entry = {
    code: diagnosticCode,
    severity: "warning",
    message,
  };
  diagnostics.push(entry);
  if (typeof document !== "undefined" && typeof document.dispatchEvent === "function" && typeof CustomEvent === "function") {
    document.dispatchEvent(new CustomEvent("glade:diagnostic", { detail: entry }));
  }
  return entry;
}

function normalizeContent(key, item = {}) {
  const contentKey = String(item.contentKey || item.key || key || "");
  return {
    contentBody: item.contentBody && typeof item.contentBody === "object" ? item.contentBody : {},
    title: String(item.title || item.name || contentKey || ""),
    urlName: String(item.urlName || contentKey || ""),
  };
}

function selectedEntry() {
  const community = readContext().community || {};
  const entries = community.managedContent || {};
  const preferred = community.contentKey || community.routeParams?.contentKey;
  if (preferred && entries[preferred]) {
    return [preferred, entries[preferred]];
  }
  return Object.entries(entries)[0] || ["", {}];
}

function editorContent() {
  const [key, item] = selectedEntry();
  return normalizeContent(key, item);
}

function editorContext() {
  const community = readContext().community || {};
  const [key, item] = selectedEntry();
  return {
    contentKey: String(item.contentKey || key || ""),
    contentSpaceId: String(item.contentSpaceId || community.contentSpaceId || community.siteId || ""),
    managedContentId: String(item.managedContentId || item.id || ""),
    schema: item.schema || { schema: { properties: {} } },
    variantId: String(item.variantId || ""),
  };
}

function unavailable(operation) {
  const body = {
    errorCode: diagnosticCode,
    message: `${operation} is hosted-only and is not persisted by local Glade LWC preview.`,
  };
  const err = new Error(body.message);
  err.body = body;
  err.status = 501;
  reportDiagnostic(body.message);
  return err;
}

function emitWire(adapter, data) {
  if (typeof adapter.dataCallback === "function") {
    adapter.dataCallback({ data, error: undefined });
  }
}

export function getContent(configOrCallback = {}) {
  if (typeof configOrCallback === "function") {
    this.dataCallback = configOrCallback;
    return;
  }
  return Promise.resolve(editorContent());
}

getContent.prototype.connect = function connect() {
  emitWire(this, editorContent());
};
getContent.prototype.disconnect = function disconnect() {};
getContent.prototype.update = getContent.prototype.connect;

export function getContext(configOrCallback = {}) {
  if (typeof configOrCallback === "function") {
    this.dataCallback = configOrCallback;
    return;
  }
  return Promise.resolve(editorContext());
}

getContext.prototype.connect = function connect() {
  emitWire(this, editorContext());
};
getContext.prototype.disconnect = function disconnect() {};
getContext.prototype.update = getContext.prototype.connect;

export function updateContent() {
  return Promise.reject(unavailable("updateContent"));
}

export default {
  getContent,
  getContext,
  updateContent,
};
