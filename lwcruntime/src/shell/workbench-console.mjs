const EVENT_KINDS = ["console", "apex", "lds", "network", "events", "issues"];
const EVENT_KIND_SET = new Set(EVENT_KINDS);

let fallbackEvents = null;
let runtimeBinding = null;
let renderScheduled = false;
const consoleRenderers = new Set();
const originalConsoleMethods = {};

export function bootWorkbenchConsole(root = document.body) {
  if (typeof document === "undefined") {
    return null;
  }
  const consoleRoot = findConsoleRoot(root);
  if (!consoleRoot) {
    return null;
  }
  if (consoleRoot.__gladeWorkbenchConsole) {
    consoleRoot.__gladeWorkbenchConsole.render();
    return consoleRoot.__gladeWorkbenchConsole;
  }
  const events = workbenchEvents();
  const render = () => renderAll(consoleRoot, events);
  const disposeTabs = bindDebugTabs(consoleRoot, render);
  const disposeEvents = bindRuntimeEvents(render);
  const controller = {
    events,
    render,
    dispose() {
      disposeTabs();
      disposeEvents();
      delete consoleRoot.__gladeWorkbenchConsole;
    },
  };
  Object.defineProperty(consoleRoot, "__gladeWorkbenchConsole", {
    configurable: true,
    value: controller,
  });
  render();
  return controller;
}

export function recordRuntimeEvent(eventOrDetail = {}) {
  const source = isDOMEvent(eventOrDetail)
    ? eventOrDetail.detail
    : eventOrDetail;
  const detail = source && typeof source === "object" ? source : { label: String(source || "Runtime event") };
  const kind = normalizeKind(detail.kind || detail.type);
  const entry = {
    time: new Date().toISOString(),
    kind,
    label: eventLabel(kind, detail),
    status: detail.status || detail.severity || "",
    detail: detail.detail !== undefined ? detail.detail : detail,
  };
  const events = workbenchEvents();
  events[kind].push(entry);
  if (kind !== "events") {
    events.events.push(entry);
  }
  dispatchRecordedEvent(entry);
  return entry;
}

function isDOMEvent(value) {
  return Boolean(value && typeof value === "object" && typeof Event !== "undefined" && value instanceof Event);
}

function findConsoleRoot(root) {
  if (!root || typeof root.querySelector !== "function") {
    return null;
  }
  if (root.matches?.("[data-glade-workbench-console]")) {
    return root;
  }
  return root.querySelector("[data-glade-workbench-console]");
}

function workbenchEvents() {
  if (typeof window === "undefined") {
    fallbackEvents = ensureEventStore(fallbackEvents);
    return fallbackEvents;
  }
  window.__gladeWorkbenchEvents = ensureEventStore(window.__gladeWorkbenchEvents);
  return window.__gladeWorkbenchEvents;
}

function ensureEventStore(store) {
  const events = store && typeof store === "object" ? store : {};
  for (const kind of EVENT_KINDS) {
    if (!Array.isArray(events[kind])) {
      events[kind] = [];
    }
  }
  return events;
}

function bindDebugTabs(root, render) {
  const tabs = Array.from(root.querySelectorAll("[data-glade-debug-tab]"));
  const panels = Array.from(root.querySelectorAll("[data-glade-debug-panel]"));
  const select = (kind) => {
    for (const tab of tabs) {
      tab.setAttribute("aria-selected", tab.dataset.gladeDebugTab === kind ? "true" : "false");
    }
    for (const panel of panels) {
      panel.hidden = panel.dataset.gladeDebugPanel !== kind;
    }
    render();
  };
  const listeners = tabs.map((tab) => {
    const onClick = () => select(tab.dataset.gladeDebugTab || "console");
    tab.addEventListener("click", onClick);
    return () => tab.removeEventListener("click", onClick);
  });
  const selected = tabs.find((tab) => tab.getAttribute("aria-selected") === "true") || tabs[0];
  if (selected) {
    select(selected.dataset.gladeDebugTab || "console");
  }
  return () => {
    for (const dispose of listeners) {
      dispose();
    }
  };
}

function bindRuntimeEvents(render) {
  if (typeof document === "undefined") {
    return () => {};
  }
  consoleRenderers.add(render);
  ensureRuntimeBinding();
  return () => {
    consoleRenderers.delete(render);
    if (consoleRenderers.size === 0 && runtimeBinding) {
      runtimeBinding.dispose();
      runtimeBinding = null;
    }
  };
}

function ensureRuntimeBinding() {
  if (runtimeBinding || typeof document === "undefined") {
    return;
  }
  const recordAndRender = (detail) => {
    recordRuntimeEvent(detail);
    scheduleRenderConsoles();
  };
  const onRuntimeEvent = (event) => recordAndRender(event);
  const onDiagnostic = (event) => recordAndRender({
    kind: "issues",
    label: diagnosticLabel(event.detail),
    status: event.detail?.severity || "warning",
    detail: event.detail || {},
  });
  const onError = (event) => recordAndRender({
    kind: "issues",
    label: event.message || "Window error",
    status: "error",
    detail: {
      message: event.message,
      filename: event.filename,
      lineno: event.lineno,
      colno: event.colno,
      error: errorMessage(event.error),
    },
  });
  const onUnhandledRejection = (event) => recordAndRender({
    kind: "issues",
    label: "Unhandled promise rejection",
    status: "error",
    detail: { reason: errorMessage(event.reason) },
  });
  document.addEventListener("glade:runtime-event", onRuntimeEvent);
  document.addEventListener("glade:diagnostic", onDiagnostic);
  installConsoleCapture(recordAndRender);
  if (typeof window !== "undefined") {
    window.addEventListener("error", onError);
    window.addEventListener("unhandledrejection", onUnhandledRejection);
  }
  runtimeBinding = {
    dispose() {
      document.removeEventListener("glade:runtime-event", onRuntimeEvent);
      document.removeEventListener("glade:diagnostic", onDiagnostic);
      uninstallConsoleCapture();
      if (typeof window !== "undefined") {
        window.removeEventListener("error", onError);
        window.removeEventListener("unhandledrejection", onUnhandledRejection);
      }
    },
  };
}

function renderConsoles() {
  for (const render of consoleRenderers) {
    render();
  }
}

function scheduleRenderConsoles() {
  if (renderScheduled) {
    return;
  }
  renderScheduled = true;
  defer(() => {
    renderScheduled = false;
    renderConsoles();
  });
}

function renderAll(root, events = workbenchEvents()) {
  for (const kind of EVENT_KINDS) {
    const output = root.querySelector(`[data-glade-debug-output="${kind}"]`);
    if (!output) {
      continue;
    }
    const entries = events[kind] || [];
    output.textContent = entries.length === 0
      ? "No events yet."
      : entries.map(formatEntry).join("\n");
  }
}

function formatEntry(entry) {
  const parts = [`[${formatTime(entry.time)}]`, entry.label || entry.kind];
  if (entry.status) {
    parts.push(`(${entry.status})`);
  }
  const detail = readableDetail(entry.detail);
  if (detail) {
    parts.push(detail);
  }
  return parts.join(" ");
}

function formatTime(value) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return String(value || "");
  }
  return date.toLocaleTimeString([], { hour12: false });
}

function readableDetail(detail) {
  if (detail === undefined || detail === null) {
    return "";
  }
  if (typeof detail === "string") {
    return detail;
  }
  try {
    return JSON.stringify(detail);
  } catch (_err) {
    return String(detail);
  }
}

function normalizeKind(kind) {
  const value = String(kind || "events").toLowerCase();
  if (value === "issue" || value === "diagnostic" || value === "diagnostics") {
    return "issues";
  }
  if (value === "event") {
    return "events";
  }
  return EVENT_KIND_SET.has(value) ? value : "events";
}

function installConsoleCapture(recordAndRender) {
  if (typeof console === "undefined") {
    return;
  }
  for (const level of ["log", "info", "warn", "error"]) {
    if (typeof console[level] !== "function" || originalConsoleMethods[level]) {
      continue;
    }
    originalConsoleMethods[level] = console[level].bind(console);
    console[level] = (...args) => {
      originalConsoleMethods[level](...args);
      recordAndRender({
        kind: "console",
        label: args.map(consoleValue).join(" ") || level,
        status: level,
        detail: { level, args: args.map(consoleValue) },
      });
    };
  }
}

function uninstallConsoleCapture() {
  if (typeof console === "undefined") {
    return;
  }
  for (const [level, method] of Object.entries(originalConsoleMethods)) {
    console[level] = method;
    delete originalConsoleMethods[level];
  }
}

function consoleValue(value) {
  if (value instanceof Error) {
    return value.stack || value.message || String(value);
  }
  if (typeof value === "string") {
    return value;
  }
  return readableDetail(value) || String(value);
}

function eventLabel(kind, detail) {
  if (detail.label) {
    return String(detail.label);
  }
  if (kind === "issues") {
    return diagnosticLabel(detail);
  }
  return detail.action || detail.message || detail.code || kind;
}

function diagnosticLabel(detail = {}) {
  const code = detail?.code ? String(detail.code) : "Diagnostic";
  const message = detail?.message ? String(detail.message) : "";
  return message ? `${code}: ${message}` : code;
}

function errorMessage(value) {
  if (!value) {
    return "";
  }
  return value.message || String(value);
}

function dispatchRecordedEvent(entry) {
  if (typeof document === "undefined" || typeof CustomEvent === "undefined") {
    return;
  }
  defer(() => {
    try {
      document.dispatchEvent(new CustomEvent("glade:workbench-event-recorded", { detail: entry }));
    } catch (_err) {
      // Runtime event collection should never affect the previewed app.
    }
  });
}

function defer(callback) {
  if (typeof queueMicrotask === "function") {
    queueMicrotask(callback);
    return;
  }
  Promise.resolve().then(callback);
}
