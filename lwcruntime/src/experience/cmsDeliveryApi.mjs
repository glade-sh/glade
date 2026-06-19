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

function contentMap() {
  return readContext().community?.managedContent || {};
}

function normalizeContent(key, item = {}) {
  if (!item) {
    return null;
  }
  const contentKey = String(item.contentKey || item.key || key || "");
  const id = String(item.id || item.managedContentId || contentKey || "");
  return {
    contentKey,
    contentType: String(item.contentType || "managedContent"),
    id,
    managedContentId: id,
    title: String(item.title || item.name || contentKey || ""),
    urlName: String(item.urlName || contentKey || ""),
    contentBody: item.contentBody && typeof item.contentBody === "object" ? item.contentBody : {},
  };
}

function findContent(config = {}) {
  const key = config.contentKeyOrId || config.contentKey || config.managedContentId || config.id;
  const entries = contentMap();
  if (key && entries[key]) {
    return normalizeContent(key, entries[key]);
  }
  if (key) {
    for (const [entryKey, item] of Object.entries(entries)) {
      if (item?.id === key || item?.managedContentId === key || item?.contentKey === key) {
        return normalizeContent(entryKey, item);
      }
    }
    return null;
  }
  const first = Object.entries(entries)[0];
  return first ? normalizeContent(first[0], first[1]) : null;
}

function listContent(config = {}) {
  const entries = contentMap();
  let items = Object.entries(entries).map(([key, item]) => normalizeContent(key, item)).filter(Boolean);
  const requestedKeys = Array.isArray(config.contentKeys) ? new Set(config.contentKeys.map(String)) : null;
  const requestedIds = Array.isArray(config.managedContentIds) ? new Set(config.managedContentIds.map(String)) : null;
  if (requestedKeys) {
    items = items.filter((item) => requestedKeys.has(item.contentKey));
  }
  if (requestedIds) {
    items = items.filter((item) => requestedIds.has(item.managedContentId) || requestedIds.has(item.id));
  }
  const page = Number(config.page || 0);
  const pageSize = Number(config.pageSize || 25);
  const start = page * pageSize;
  const pageItems = items.slice(start, start + pageSize);
  return {
    currentPage: page,
    items: pageItems,
    pageSize,
    total: items.length,
    totalPages: items.length === 0 ? 0 : Math.ceil(items.length / pageSize),
  };
}

function emitWire(adapter, data) {
  if (typeof adapter.dataCallback === "function") {
    adapter.dataCallback({ data, error: undefined });
  }
}

export function getContent(configOrCallback = {}) {
  if (typeof configOrCallback === "function") {
    this.dataCallback = configOrCallback;
    this.config = {};
    return;
  }
  return Promise.resolve(findContent(configOrCallback));
}

getContent.prototype.connect = function connect() {
  emitWire(this, findContent(this.config));
};
getContent.prototype.disconnect = function disconnect() {};
getContent.prototype.update = function update(config = {}) {
  this.config = config;
  emitWire(this, findContent(config));
};

export function getContents(configOrCallback = {}) {
  if (typeof configOrCallback === "function") {
    this.dataCallback = configOrCallback;
    this.config = {};
    return;
  }
  return Promise.resolve(listContent(configOrCallback));
}

getContents.prototype.connect = function connect() {
  emitWire(this, listContent(this.config));
};
getContents.prototype.disconnect = function disconnect() {};
getContents.prototype.update = function update(config = {}) {
  this.config = config;
  emitWire(this, listContent(config));
};

export default {
  getContent,
  getContents,
};
