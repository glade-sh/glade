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

function contextObjectApiName() {
  const context = readJSON("glade-lwc-context");
  return context.objectApiName || context.ObjectAPIName || "";
}

function listReference(config = {}) {
  return {
    objectApiName: config.objectApiName || contextObjectApiName() || null,
    listViewApiName: config.listViewApiName || config.listViewApiNameOrId || config.listViewName || null,
  };
}

function listLabel(reference) {
  return reference.listViewApiName || "All";
}

function emptyListInfo(config = {}) {
  const reference = listReference(config);
  return {
    displayColumns: [],
    filteredByInfo: [],
    label: listLabel(reference),
    listReference: reference,
    orderBy: [],
    scope: null,
    visibility: "Public",
  };
}

function listInfosPayload(config = {}) {
  const objectApiName = config.objectApiName || contextObjectApiName() || null;
  const listInfos = objectApiName ? [emptyListInfo({ objectApiName, listViewApiName: "All" })] : [];
  return {
    count: listInfos.length,
    listInfos,
  };
}

function emptyPage(extra = {}) {
  return {
    count: 0,
    currentPageToken: null,
    nextPageToken: null,
    previousPageToken: null,
    ...extra,
  };
}

function unsupportedWriteError(operation) {
  const body = {
    errorCode: "GLADELWC091",
    message: `${operation} is not supported by the local uiListsApi shim`,
  };
  const err = new Error(body.message);
  err.body = body;
  err.status = 501;
  return err;
}

function createReadAdapter(readPayload) {
  function adapter(configOrCallback = {}) {
    if (typeof configOrCallback === "function") {
      this.dataCallback = configOrCallback;
      this.config = {};
      return undefined;
    }
    return Promise.resolve(readPayload(configOrCallback || {}));
  }
  adapter.prototype.connect = function connect() {
    if (typeof this.dataCallback === "function") {
      this.dataCallback({ data: readPayload(this.config || {}), error: undefined });
    }
  };
  adapter.prototype.disconnect = function disconnect() {};
  adapter.prototype.update = function update(config = {}) {
    this.config = config || {};
    this.connect();
  };
  return adapter;
}

export const getListInfosByObjectName = createReadAdapter((config) => emptyPage(listInfosPayload(config)));

export const getListInfoByName = createReadAdapter((config) => emptyListInfo(config));

export const getListRecordsByName = createReadAdapter(() => emptyPage({ records: [] }));

export const getListPreferences = createReadAdapter(() => ({
  columnWidths: {},
  wrapText: false,
}));

export function updateListInfoByName() {
  return Promise.reject(unsupportedWriteError("updateListInfoByName"));
}
