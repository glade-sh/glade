const EVENT_KINDS = ["console", "apex", "lds", "network", "events", "issues"];
const EVENT_KIND_SET = new Set(EVENT_KINDS);
const STORAGE_KEY = "glade:workbench-console:v2";
const RUN_POLL_INTERVAL_MS = 1000;
const MAX_ENTRIES_PER_KIND = 200;
const MAX_RUNS = 25;
const DEBUG_DOCK_MIN_HEIGHT = 150;
const DEBUG_DOCK_MINIMIZED_HEIGHT = 36;
const DEBUG_DOCK_MIN_PREVIEW_HEIGHT = 180;
const DEBUG_DOCK_RESIZE_STEP = 24;

let fallbackEvents = null;
let runtimeBinding = null;
let renderScheduled = false;
let persistScheduled = false;
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
  ensureRunMonitor(consoleRoot);
  ensureDebugControls(consoleRoot);
  ensureDebugResizeHandle(consoleRoot);
  const events = workbenchEvents();
  const render = () => renderAll(consoleRoot, events);
  const disposeTabs = bindDebugTabs(consoleRoot, render, events);
  const disposeControls = bindDebugControls(consoleRoot, render, events);
  const disposeResize = bindDebugResize(consoleRoot, render, events);
  const disposeEvents = bindRuntimeEvents(render);
  const disposeRuns = bindDevRuns(render, events);
  const controller = {
    events,
    render,
    dispose() {
      disposeTabs();
      disposeControls();
      disposeResize();
      disposeEvents();
      disposeRuns();
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
  const events = workbenchEvents();
  const entry = {
    time: new Date().toISOString(),
    kind,
    runId: String(detail.runId || events.activeRunId || ""),
    label: eventLabel(kind, detail),
    status: detail.status || detail.severity || "",
    detail: detail.detail !== undefined ? detail.detail : detail,
  };
  pushEntry(events, kind, entry);
  if (kind !== "events") {
    pushEntry(events, "events", entry);
  }
  persistWorkbenchEvents(events);
  dispatchRecordedEvent(entry);
  return entry;
}

export function recordDevRunEvent(eventOrDetail = {}) {
  const source = isDOMEvent(eventOrDetail)
    ? eventOrDetail.detail
    : eventOrDetail;
  const detail = source && typeof source === "object" ? source : {};
  const events = workbenchEvents();
  const run = normalizeRunEvent(detail, events);
  upsertRun(events, run);
  events.activeRunId = run.id;
  if (Number.isFinite(run.sequence)) {
    events.latestSequence = Math.max(events.latestSequence || 0, run.sequence);
  }
  const entry = {
    time: run.finishedAt || run.startedAt || new Date().toISOString(),
    kind: "events",
    runId: run.id,
    label: `Save run ${run.id}`,
    status: run.status,
    detail: {
      label: run.label,
      changedFiles: run.changedFiles,
      durationMs: run.durationMs,
      error: run.error,
    },
  };
  pushEntry(events, "events", entry);
  persistWorkbenchEvents(events);
  dispatchRecordedEvent(entry);
  return run;
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
  if (!window.__gladeWorkbenchEvents) {
    window.__gladeWorkbenchEvents = loadPersistedWorkbenchEvents();
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
  if (!Array.isArray(events.runs)) {
    events.runs = [];
  }
  events.activeRunId = String(events.activeRunId || "");
  events.selectedTab = String(events.selectedTab || "");
  events.filterQuery = String(events.filterQuery || "");
  events.problemsOnly = Boolean(events.problemsOnly);
  events.latestSequence = Number.isFinite(Number(events.latestSequence)) ? Number(events.latestSequence) : 0;
  events.debugDockHeight = Number.isFinite(Number(events.debugDockHeight)) ? Number(events.debugDockHeight) : 0;
  events.debugDockMinimized = Object.prototype.hasOwnProperty.call(events, "debugDockMinimized")
    ? Boolean(events.debugDockMinimized)
    : true;
  return events;
}

function loadPersistedWorkbenchEvents() {
  if (typeof sessionStorage === "undefined") {
    return {};
  }
  try {
    return JSON.parse(sessionStorage.getItem(STORAGE_KEY) || "{}");
  } catch (_err) {
    return {};
  }
}

function persistWorkbenchEvents(events = workbenchEvents(), options = {}) {
  if (options.immediate) {
    writePersistedWorkbenchEvents(events);
    return;
  }
  if (persistScheduled) {
    return;
  }
  persistScheduled = true;
  defer(() => {
    persistScheduled = false;
    writePersistedWorkbenchEvents(events);
  });
}

function writePersistedWorkbenchEvents(events = workbenchEvents()) {
  if (typeof sessionStorage === "undefined") {
    return;
  }
  try {
    sessionStorage.setItem(STORAGE_KEY, JSON.stringify({
      runs: events.runs || [],
      activeRunId: events.activeRunId || "",
      selectedTab: events.selectedTab || "",
      filterQuery: events.filterQuery || "",
      problemsOnly: Boolean(events.problemsOnly),
      latestSequence: events.latestSequence || 0,
      debugDockHeight: events.debugDockHeight || 0,
      debugDockMinimized: Boolean(events.debugDockMinimized),
      console: events.console || [],
      apex: events.apex || [],
      lds: events.lds || [],
      network: events.network || [],
      events: events.events || [],
      issues: events.issues || [],
    }));
  } catch (_err) {
    // Console persistence should never affect the preview.
  }
}

function bindDebugTabs(root, render, events) {
  const tabs = Array.from(root.querySelectorAll("[data-glade-debug-tab]"));
  const panels = Array.from(root.querySelectorAll("[data-glade-debug-panel]"));
  const select = (kind) => {
    const selectedKind = EVENT_KIND_SET.has(kind) ? kind : "console";
    events.selectedTab = selectedKind;
    for (const tab of tabs) {
      tab.setAttribute("aria-selected", tab.dataset.gladeDebugTab === selectedKind ? "true" : "false");
    }
    for (const panel of panels) {
      panel.hidden = panel.dataset.gladeDebugPanel !== selectedKind;
    }
    persistWorkbenchEvents(events);
    render();
  };
  const listeners = tabs.map((tab) => {
    const onClick = () => select(tab.dataset.gladeDebugTab || "console");
    tab.addEventListener("click", onClick);
    return () => tab.removeEventListener("click", onClick);
  });
  const selected = tabs.find((tab) => tab.dataset.gladeDebugTab === events.selectedTab)
    || tabs.find((tab) => tab.getAttribute("aria-selected") === "true")
    || tabs[0];
  if (selected) {
    select(selected.dataset.gladeDebugTab || "console");
  }
  return () => {
    for (const dispose of listeners) {
      dispose();
    }
  };
}

function bindDebugControls(root, render, events) {
  const filter = root.querySelector("[data-glade-debug-filter]");
  const problems = root.querySelector("[data-glade-debug-problems]");
  const clearCurrent = root.querySelector("[data-glade-debug-clear-current]");
  const clearAll = root.querySelector("[data-glade-debug-clear-all]");
  const disposers = [];
  if (filter) {
    const onInput = () => {
      events.filterQuery = filter.value;
      persistWorkbenchEvents(events);
      render();
    };
    filter.addEventListener("input", onInput);
    disposers.push(() => filter.removeEventListener("input", onInput));
  }
  if (problems) {
    const onClick = () => {
      events.problemsOnly = !events.problemsOnly;
      persistWorkbenchEvents(events);
      render();
    };
    problems.addEventListener("click", onClick);
    disposers.push(() => problems.removeEventListener("click", onClick));
  }
  if (clearCurrent) {
    const onClick = () => {
      const kind = EVENT_KIND_SET.has(events.selectedTab) ? events.selectedTab : "console";
      events[kind] = [];
      persistWorkbenchEvents(events);
      render();
    };
    clearCurrent.addEventListener("click", onClick);
    disposers.push(() => clearCurrent.removeEventListener("click", onClick));
  }
  if (clearAll) {
    const onClick = () => {
      clearWorkbenchEvents(events);
      persistWorkbenchEvents(events);
      render();
    };
    clearAll.addEventListener("click", onClick);
    disposers.push(() => clearAll.removeEventListener("click", onClick));
  }
  return () => {
    for (const dispose of disposers) {
      dispose();
    }
  };
}

function bindDebugResize(root, render, events) {
  if (typeof document === "undefined") {
    return () => {};
  }
  const dock = root.querySelector("[data-glade-debug-dock]");
  const handle = root.querySelector("[data-glade-debug-resize-handle]");
  const minimize = root.querySelector("[data-glade-debug-minimize]");
  const restore = root.querySelector("[data-glade-debug-restore]");
  if (!dock || !handle) {
    return () => {};
  }
  let drag = null;
  const startDrag = (event) => {
    if (event.button !== undefined && event.button !== 0) {
      return;
    }
    event.preventDefault();
    drag = {
      pointerId: event.pointerId,
      startY: event.clientY,
      startHeight: dock.getBoundingClientRect().height,
    };
    dock.dataset.gladeDebugResizing = "true";
    handle.setPointerCapture?.(event.pointerId);
  };
  const moveDrag = (event) => {
    if (!drag || event.pointerId !== drag.pointerId) {
      return;
    }
    event.preventDefault();
    events.debugDockMinimized = false;
    events.debugDockHeight = clampDebugDockHeight(root, drag.startHeight + drag.startY - event.clientY);
    applyDebugDockState(root, events);
  };
  const finishDrag = (event) => {
    if (!drag || (event.pointerId !== undefined && event.pointerId !== drag.pointerId)) {
      return;
    }
    try {
      handle.releasePointerCapture?.(drag.pointerId);
    } catch (_err) {
      // Pointer capture can already be gone after cancel/lostpointercapture.
    }
    delete dock.dataset.gladeDebugResizing;
    drag = null;
    persistWorkbenchEvents(events);
    render();
  };
  const adjustHeight = (delta) => {
    events.debugDockMinimized = false;
    events.debugDockHeight = clampDebugDockHeight(root, currentDebugDockHeight(root, dock, events) + delta);
    applyDebugDockState(root, events);
    persistWorkbenchEvents(events);
    render();
  };
  const setMinHeight = () => {
    events.debugDockMinimized = false;
    events.debugDockHeight = debugDockBounds(root).min;
    applyDebugDockState(root, events);
    persistWorkbenchEvents(events);
    render();
  };
  const setMaxHeight = () => {
    events.debugDockMinimized = false;
    events.debugDockHeight = debugDockBounds(root).max;
    applyDebugDockState(root, events);
    persistWorkbenchEvents(events);
    render();
  };
  const onKeyDown = (event) => {
    if (event.key === "ArrowUp") {
      event.preventDefault();
      adjustHeight(DEBUG_DOCK_RESIZE_STEP);
    } else if (event.key === "ArrowDown") {
      event.preventDefault();
      adjustHeight(-DEBUG_DOCK_RESIZE_STEP);
    } else if (event.key === "Home") {
      event.preventDefault();
      setMinHeight();
    } else if (event.key === "End") {
      event.preventDefault();
      setMaxHeight();
    } else if (event.key === "Enter" || event.key === " ") {
      event.preventDefault();
      if (events.debugDockMinimized) {
        restoreDock(root, events);
      } else {
        minimizeDock(root, events);
      }
      persistWorkbenchEvents(events);
      render();
    }
  };
  const onMinimize = () => {
    minimizeDock(root, events);
    persistWorkbenchEvents(events);
    render();
  };
  const onRestore = () => {
    restoreDock(root, events);
    persistWorkbenchEvents(events);
    render();
  };
  handle.addEventListener("pointerdown", startDrag);
  handle.addEventListener("pointermove", moveDrag);
  handle.addEventListener("pointerup", finishDrag);
  handle.addEventListener("pointercancel", finishDrag);
  handle.addEventListener("lostpointercapture", finishDrag);
  handle.addEventListener("keydown", onKeyDown);
  minimize?.addEventListener("click", onMinimize);
  restore?.addEventListener("click", onRestore);
  applyDebugDockState(root, events);
  return () => {
    handle.removeEventListener("pointerdown", startDrag);
    handle.removeEventListener("pointermove", moveDrag);
    handle.removeEventListener("pointerup", finishDrag);
    handle.removeEventListener("pointercancel", finishDrag);
    handle.removeEventListener("lostpointercapture", finishDrag);
    handle.removeEventListener("keydown", onKeyDown);
    minimize?.removeEventListener("click", onMinimize);
    restore?.removeEventListener("click", onRestore);
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

function bindDevRuns(render, events) {
  if (typeof document === "undefined") {
    return () => {};
  }
  const onDevRun = (event) => {
    const run = recordDevRunEvent(event);
    render();
    if (run.reload) {
      schedulePreviewReload(events, run);
    }
  };
  document.addEventListener("glade:dev-run", onDevRun);
  let disposed = false;
  let timer = 0;
  const poll = async () => {
    if (disposed || typeof fetch !== "function") {
      return;
    }
    try {
      const response = await fetch(`/lightning/local/runs.json?since=${encodeURIComponent(String(events.latestSequence || 0))}`, {
        headers: { Accept: "application/json" },
      });
      if (response.ok) {
        const payload = await response.json();
        for (const run of payload.runs || []) {
          const recorded = recordDevRunEvent(run);
          if (recorded.reload) {
            schedulePreviewReload(events, recorded);
          }
        }
        if (Number.isFinite(Number(payload.latestSequence))) {
          events.latestSequence = Math.max(events.latestSequence || 0, Number(payload.latestSequence));
        }
        persistWorkbenchEvents(events);
        render();
      }
    } catch (_err) {
      // The fixture server and some static previews do not expose run polling.
    } finally {
      if (!disposed) {
        timer = window.setTimeout(poll, RUN_POLL_INTERVAL_MS);
      }
    }
  };
  timer = window.setTimeout(poll, RUN_POLL_INTERVAL_MS);
  if (typeof window !== "undefined") {
    window.addEventListener("beforeunload", persistOnUnload);
  }
  return () => {
    disposed = true;
    document.removeEventListener("glade:dev-run", onDevRun);
    if (timer) {
      window.clearTimeout(timer);
    }
    if (typeof window !== "undefined") {
      window.removeEventListener("beforeunload", persistOnUnload);
    }
  };
}

function persistOnUnload() {
  persistWorkbenchEvents(workbenchEvents(), { immediate: true });
}

function schedulePreviewReload(events, run) {
  if (typeof window === "undefined" || run.__reloadScheduled) {
    return;
  }
  run.__reloadScheduled = true;
  persistWorkbenchEvents(events, { immediate: true });
  window.setTimeout(() => {
    window.location.reload();
  }, 150);
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
  applyDebugDockState(root, events);
  renderRunMonitor(root, events);
  renderDebugControls(root, events);
  const selectedKind = EVENT_KIND_SET.has(events.selectedTab) ? events.selectedTab : "console";
  for (const kind of EVENT_KINDS) {
    const output = root.querySelector(`[data-glade-debug-output="${kind}"]`);
    if (!output) {
      continue;
    }
    if (kind !== selectedKind) {
      if (output.firstChild) {
        output.replaceChildren();
      }
      continue;
    }
    const entries = events[kind] || [];
    renderDebugOutput(output, entries, events);
  }
}

function ensureRunMonitor(root) {
  const dock = root.querySelector("[data-glade-debug-dock]");
  if (!dock || dock.querySelector("[data-glade-run-summary]")) {
    return;
  }
  const monitor = document.createElement("section");
  monitor.className = "glade-run-monitor";
  monitor.setAttribute("aria-label", "Save run status");
  monitor.innerHTML = `<div class="glade-run-summary" data-glade-run-summary></div><ol class="glade-run-timeline" data-glade-run-timeline></ol>`;
  dock.prepend(monitor);
}

function ensureDebugControls(root) {
  const dock = root.querySelector("[data-glade-debug-dock]");
  if (!dock || dock.querySelector("[data-glade-debug-tools]")) {
    return;
  }
  const tools = document.createElement("section");
  tools.className = "glade-debug-tools";
  tools.dataset.gladeDebugTools = "";
  tools.setAttribute("aria-label", "Debug console tools");
  tools.innerHTML = `<label class="glade-debug-filter"><span>Filter</span><input type="search" data-glade-debug-filter placeholder="Search and highlight output" autocomplete="off"></label><button type="button" data-glade-debug-problems aria-pressed="false">Problems</button><button type="button" data-glade-debug-clear-current>Clear view</button><button type="button" data-glade-debug-clear-all>Clear all</button><button class="glade-debug-minimize" type="button" data-glade-debug-minimize aria-label="Collapse console" title="Collapse console">↓</button>`;
  const tabs = dock.querySelector("[role='tablist']");
  if (tabs) {
    tabs.before(tools);
  } else {
    dock.append(tools);
  }
}

function ensureDebugResizeHandle(root) {
  const dock = root.querySelector("[data-glade-debug-dock]");
  if (!dock || dock.querySelector("[data-glade-debug-resize-shell]")) {
    return;
  }
  const shell = document.createElement("section");
  shell.className = "glade-debug-resize-shell";
  shell.dataset.gladeDebugResizeShell = "";
  shell.setAttribute("aria-label", "Console dock controls");
  shell.innerHTML = `<div class="glade-debug-resize-grip" data-glade-debug-resize-handle role="separator" tabindex="0" aria-orientation="horizontal" aria-label="Resize console" aria-valuemin="${DEBUG_DOCK_MIN_HEIGHT}" aria-valuemax="${DEBUG_DOCK_MIN_HEIGHT}" aria-valuenow="0"><span></span></div><div class="glade-debug-minimized-bar" data-glade-debug-minimized-bar><span>Console</span><button type="button" data-glade-debug-restore aria-label="Expand console" title="Expand console">↑</button></div>`;
  dock.prepend(shell);
}

function renderDebugControls(root, events) {
  const filter = root.querySelector("[data-glade-debug-filter]");
  if (filter && filter.value !== events.filterQuery) {
    filter.value = events.filterQuery || "";
  }
  const problems = root.querySelector("[data-glade-debug-problems]");
  if (problems) {
    problems.setAttribute("aria-pressed", events.problemsOnly ? "true" : "false");
    problems.dataset.gladeSelected = events.problemsOnly ? "true" : "false";
  }
}

function minimizeDock(root, events) {
  const stored = Number(events.debugDockHeight);
  if (!Number.isFinite(stored) || stored <= 0) {
    events.debugDockHeight = 0;
  }
  events.debugDockMinimized = true;
  applyDebugDockState(root, events);
}

function restoreDock(root, events) {
  events.debugDockMinimized = false;
  applyDebugDockState(root, events);
}

function applyDebugDockState(root, events) {
  const dock = root.querySelector("[data-glade-debug-dock]");
  const handle = root.querySelector("[data-glade-debug-resize-handle]");
  if (!dock) {
    return;
  }
  const bounds = debugDockBounds(root);
  if (events.debugDockMinimized) {
    root.style.setProperty("--glade-debug-dock-height", `${DEBUG_DOCK_MINIMIZED_HEIGHT}px`);
    dock.dataset.gladeDebugMinimized = "true";
    updateDebugDockHandle(handle, bounds, DEBUG_DOCK_MINIMIZED_HEIGHT);
    return;
  }
  delete dock.dataset.gladeDebugMinimized;
  const requested = Number(events.debugDockHeight);
  if (Number.isFinite(requested) && requested > 0) {
    const height = clampDebugDockHeight(root, requested);
    events.debugDockHeight = height;
    root.style.setProperty("--glade-debug-dock-height", `${height}px`);
    const renderedHeight = Math.round(dock.getBoundingClientRect().height || height);
    updateDebugDockHandle(handle, bounds, renderedHeight);
    return;
  }
  root.style.removeProperty("--glade-debug-dock-height");
  updateDebugDockHandle(handle, bounds, Math.round(dock.getBoundingClientRect().height || 0));
}

function updateDebugDockHandle(handle, bounds, value) {
  if (!handle) {
    return;
  }
  handle.setAttribute("aria-valuemin", String(bounds.min));
  handle.setAttribute("aria-valuemax", String(bounds.max));
  handle.setAttribute("aria-valuenow", String(Math.round(value || 0)));
}

function currentDebugDockHeight(root, dock, events) {
  const current = dock.getBoundingClientRect().height;
  if (current > DEBUG_DOCK_MINIMIZED_HEIGHT + 8) {
    return current;
  }
  const stored = Number(events.debugDockHeight);
  if (Number.isFinite(stored) && stored > 0) {
    return stored;
  }
  return debugDockBounds(root).min;
}

function clampDebugDockHeight(root, value) {
  const bounds = debugDockBounds(root);
  const height = Number(value);
  if (!Number.isFinite(height)) {
    return bounds.min;
  }
  return Math.min(bounds.max, Math.max(bounds.min, Math.round(height)));
}

function debugDockBounds(root) {
  const rootHeight = root.getBoundingClientRect().height
    || (typeof window !== "undefined" ? window.innerHeight : 0)
    || DEBUG_DOCK_MIN_HEIGHT + DEBUG_DOCK_MIN_PREVIEW_HEIGHT;
  const max = Math.max(DEBUG_DOCK_MIN_HEIGHT, Math.round(rootHeight - DEBUG_DOCK_MIN_PREVIEW_HEIGHT));
  return {
    min: Math.min(DEBUG_DOCK_MIN_HEIGHT, max),
    max,
  };
}

function renderDebugOutput(output, entries, events) {
  const displayEntries = filteredEntries(entries, events);
  const emptyText = entries.length === 0 ? "No events yet." : "No matching events.";
  output.replaceChildren();
  if (displayEntries.length === 0) {
    output.textContent = emptyText;
    return;
  }
  const query = normalizedFilterQuery(events);
  displayEntries.forEach((entry, index) => {
    const line = document.createElement("span");
    line.className = "glade-debug-line";
    const status = entryStatus(entry);
    if (status) {
      line.dataset.gladeEntryStatus = status;
    }
    appendSyntaxHighlightedEntry(line, entry, query);
    output.append(line);
    if (index < displayEntries.length - 1) {
      output.append(document.createTextNode("\n"));
    }
  });
}

function renderRunMonitor(root, events) {
  const summary = root.querySelector("[data-glade-run-summary]");
  const timeline = root.querySelector("[data-glade-run-timeline]");
  if (!summary || !timeline) {
    return;
  }
  const monitor = summary.closest(".glade-run-monitor");
  const latest = latestRun(events);
  if (!latest) {
    summary.textContent = "";
    timeline.replaceChildren();
    if (monitor) {
      monitor.hidden = true;
    }
    return;
  }
  if (monitor) {
    monitor.hidden = false;
  }
  const issues = issuesForRun(events, latest.id);
  summary.textContent = [
    runStatusLabel(latest, issues),
    latest.label || "Save run",
    durationText(latest.durationMs),
    `${issues.length} error${issues.length === 1 ? "" : "s"}`,
    shortFileList(latest.changedFiles),
  ].filter(Boolean).join(" · ");
  timeline.replaceChildren();
  for (const run of [...events.runs].slice(-5).reverse()) {
    const item = document.createElement("li");
    item.dataset.gladeRunId = run.id;
    item.textContent = [
      run.id,
      run.status,
      durationText(run.durationMs),
      shortFileList(run.changedFiles),
    ].filter(Boolean).join(" · ");
    timeline.append(item);
  }
}

function filteredEntries(entries, events) {
  const query = normalizedFilterQuery(events);
  return entries.filter((entry) => {
    if (events.problemsOnly && !isProblemEntry(entry)) {
      return false;
    }
    if (!query) {
      return true;
    }
    return formatEntry(entry).toLowerCase().includes(query);
  });
}

function normalizedFilterQuery(events) {
  return String(events.filterQuery || "").trim().toLowerCase();
}

function isProblemEntry(entry) {
  if (entry.kind === "issues") {
    return true;
  }
  const status = entryStatus(entry);
  return status === "error" || status === "warn" || status === "warning" || status === "fail" || status === "failed";
}

function entryStatus(entry) {
  return String(entry.status || entry.detail?.severity || entry.detail?.level || "").toLowerCase();
}

function appendSyntaxHighlightedEntry(target, entry, query) {
  for (const segment of entrySyntaxSegments(entry)) {
    appendSyntaxSegment(target, segment, query);
  }
}

function appendSyntaxSegment(target, segment, query) {
  const text = String(segment.text ?? "");
  if (!text) {
    return;
  }
  if (!segment.token) {
    target.append(document.createTextNode(text));
    return;
  }
  const span = document.createElement("span");
  span.className = "glade-debug-token";
  span.dataset.gladeToken = segment.token;
  appendHighlightedText(span, text, query);
  target.append(span);
}

function entrySyntaxSegments(entry) {
  if (entry.kind === "apex") {
    return apexEntrySegments(entry);
  }
  if (entry.kind === "lds") {
    return ldsEntrySegments(entry);
  }
  return genericEntrySegments(entry);
}

function apexEntrySegments(entry) {
  const detail = entry.detail || {};
  const method = detail.className && detail.method ? `${detail.className}.${detail.method}` : entry.label;
  const segments = prefixSegments(entry);
  appendToken(segments, "method", method);
  appendStatusSegments(segments, entry.status || "apex");
  appendText(segments, " ");
  appendToken(segments, "field", "params");
  appendText(segments, "=");
  appendDetailSegments(segments, detail.params || {});
  if (detail.result !== undefined) {
    appendText(segments, " -> ");
    appendDetailSegments(segments, detail.result);
  }
  if (detail.body !== undefined || detail.error) {
    appendText(segments, " -> ");
    appendToken(segments, "error", "ERROR");
    appendText(segments, " ");
    appendDetailSegments(segments, detail.body || detail.error);
  }
  appendDurationSegments(segments, detail.durationMs);
  return segments;
}

function ldsEntrySegments(entry) {
  const detail = entry.detail || {};
  const label = entry.label || detail.endpoint || detail.action || "LDS";
  const segments = prefixSegments(entry);
  appendToken(segments, detail.endpoint ? "endpoint" : labelToken(label), label);
  appendStatusSegments(segments, entry.status);
  if (detail.body !== undefined) {
    appendText(segments, " ");
    appendToken(segments, "field", "input");
    appendText(segments, "=");
    appendDetailSegments(segments, detail.body);
  } else if (detail.recordIds !== undefined) {
    appendText(segments, " ");
    appendToken(segments, "field", "records");
    appendText(segments, "=");
    appendDetailSegments(segments, detail.recordIds);
  } else if (detail.key !== undefined) {
    appendText(segments, " ");
    appendToken(segments, "field", "key");
    appendText(segments, "=");
    appendToken(segments, "json-string", String(detail.key));
  }
  if (detail.result !== undefined) {
    appendText(segments, " -> ");
    appendDetailSegments(segments, detail.result);
  }
  appendDurationSegments(segments, detail.durationMs);
  return segments;
}

function genericEntrySegments(entry) {
  const label = entry.label || entry.kind;
  const segments = prefixSegments(entry);
  appendToken(segments, labelToken(label), label);
  appendStatusSegments(segments, entry.status);
  if (entry.detail !== undefined && entry.detail !== null) {
    const detailText = readableDetail(entry.detail);
    if (detailText) {
      appendText(segments, " ");
      appendDetailSegments(segments, entry.detail);
    }
  }
  return segments;
}

function prefixSegments(entry) {
  const segments = [];
  appendText(segments, "[");
  appendToken(segments, "time", formatTime(entry.time));
  appendText(segments, "]");
  const run = runPrefix(entry);
  if (run) {
    appendText(segments, " ");
    appendToken(segments, "run", run);
  }
  appendText(segments, " ");
  return segments;
}

function appendStatusSegments(segments, status) {
  if (!status) {
    return;
  }
  appendText(segments, " (");
  appendToken(segments, "status", status);
  appendText(segments, ")");
}

function appendDurationSegments(segments, value) {
  const duration = durationText(value);
  if (!duration) {
    return;
  }
  appendText(segments, " ");
  appendToken(segments, "duration", duration);
}

function appendDetailSegments(segments, detail) {
  if (detail === undefined || detail === null) {
    if (detail === null) {
      appendToken(segments, "json-null", "null");
    }
    return;
  }
  if (typeof detail === "string") {
    appendToken(segments, "json-string", detail);
    return;
  }
  if (typeof detail === "number") {
    appendToken(segments, "json-number", String(detail));
    return;
  }
  if (typeof detail === "boolean") {
    appendToken(segments, "json-boolean", String(detail));
    return;
  }
  if (Array.isArray(detail)) {
    appendText(segments, "[");
    detail.forEach((item, index) => {
      if (index > 0) {
        appendText(segments, ",");
      }
      appendDetailSegments(segments, item);
    });
    appendText(segments, "]");
    return;
  }
  if (typeof detail === "object") {
    const entries = Object.entries(detail);
    appendText(segments, "{");
    entries.forEach(([key, value], index) => {
      if (index > 0) {
        appendText(segments, ",");
      }
      appendToken(segments, "json-key", JSON.stringify(key));
      appendText(segments, ":");
      appendDetailSegments(segments, value);
    });
    appendText(segments, "}");
    return;
  }
  appendText(segments, String(detail));
}

function labelToken(label) {
  const value = String(label || "");
  if (value.startsWith("/") || value.startsWith("http://") || value.startsWith("https://")) {
    return "endpoint";
  }
  if (/^[A-Za-z_$][\w$]*\.[A-Za-z_$][\w$]*$/.test(value)) {
    return "method";
  }
  return "label";
}

function appendText(segments, text) {
  if (text !== undefined && text !== null && text !== "") {
    segments.push({ text: String(text) });
  }
}

function appendToken(segments, token, text) {
  if (text !== undefined && text !== null && text !== "") {
    segments.push({ token, text: String(text) });
  }
}

function appendHighlightedText(target, text, query) {
  if (!query) {
    target.append(document.createTextNode(text));
    return;
  }
  const lower = text.toLowerCase();
  let start = 0;
  for (;;) {
    const index = lower.indexOf(query, start);
    if (index < 0) {
      break;
    }
    if (index > start) {
      target.append(document.createTextNode(text.slice(start, index)));
    }
    const mark = document.createElement("mark");
    mark.dataset.gladeDebugMatch = "";
    mark.textContent = text.slice(index, index + query.length);
    target.append(mark);
    start = index + query.length;
  }
  if (start < text.length) {
    target.append(document.createTextNode(text.slice(start)));
  }
}

function clearWorkbenchEvents(events) {
  for (const kind of EVENT_KINDS) {
    events[kind] = [];
  }
  events.runs = [];
  events.activeRunId = "";
  events.selectedTab = EVENT_KIND_SET.has(events.selectedTab) ? events.selectedTab : "console";
  events.filterQuery = "";
  events.problemsOnly = false;
}

function latestRun(events) {
  if (!events.runs?.length) {
    return null;
  }
  return events.runs[events.runs.length - 1];
}

function issuesForRun(events, runId) {
  return (events.issues || []).filter((entry) => {
    if (runId && entry.runId !== runId) {
      return false;
    }
    const status = String(entry.status || entry.detail?.severity || "").toLowerCase();
    return status === "error";
  });
}

function runStatusLabel(run, issues) {
  if (run.status === "running") {
    return "RUNNING";
  }
  if (run.status === "error" || run.error || issues.length > 0) {
    return "FAIL";
  }
  if (run.status === "success") {
    return "PASS";
  }
  return String(run.status || "RUN");
}

function shortFileList(files = []) {
  const names = files.map((file) => String(file).split("/").pop()).filter(Boolean);
  if (names.length === 0) {
    return "";
  }
  if (names.length <= 2) {
    return names.join(", ");
  }
  return `${names.slice(0, 2).join(", ")} +${names.length - 2}`;
}

function formatEntry(entry) {
  if (entry.kind === "apex") {
    return formatApexEntry(entry);
  }
  if (entry.kind === "lds") {
    return formatLDSEntry(entry);
  }
  const parts = [`[${formatTime(entry.time)}]`, runPrefix(entry), entry.label || entry.kind];
  if (entry.status) {
    parts.push(`(${entry.status})`);
  }
  const detail = readableDetail(entry.detail);
  if (detail) {
    parts.push(detail);
  }
  return parts.filter(Boolean).join(" ");
}

function formatApexEntry(entry) {
  const detail = entry.detail || {};
  const method = detail.className && detail.method ? `${detail.className}.${detail.method}` : entry.label;
  const parts = [`[${formatTime(entry.time)}]`, runPrefix(entry), method];
  parts.push(`(${entry.status || "apex"})`);
  parts.push(`params=${readableDetail(detail.params || {})}`);
  if (detail.result !== undefined) {
    parts.push(`-> ${readableDetail(detail.result)}`);
  }
  if (detail.body !== undefined || detail.error) {
    parts.push(`-> ERROR ${readableDetail(detail.body || detail.error)}`);
  }
  const duration = durationText(detail.durationMs);
  if (duration) {
    parts.push(duration);
  }
  return parts.filter(Boolean).join(" ");
}

function formatLDSEntry(entry) {
  const detail = entry.detail || {};
  const label = entry.label || detail.endpoint || detail.action || "LDS";
  const parts = [`[${formatTime(entry.time)}]`, runPrefix(entry), label];
  if (entry.status) {
    parts.push(`(${entry.status})`);
  }
  if (detail.body !== undefined) {
    parts.push(`input=${readableDetail(detail.body)}`);
  } else if (detail.recordIds !== undefined) {
    parts.push(`records=${readableDetail(detail.recordIds)}`);
  } else if (detail.key !== undefined) {
    parts.push(`key=${detail.key}`);
  }
  if (detail.result !== undefined) {
    parts.push(`-> ${readableDetail(detail.result)}`);
  }
  const duration = durationText(detail.durationMs);
  if (duration) {
    parts.push(duration);
  }
  return parts.filter(Boolean).join(" ");
}

function runPrefix(entry) {
  return entry.runId ? `#${entry.runId}` : "";
}

function durationText(value) {
  const number = Number(value);
  if (!Number.isFinite(number) || number <= 0) {
    return "";
  }
  return `${Math.round(number)}ms`;
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

function normalizeRunEvent(detail, events) {
  const id = String(detail.id || detail.runId || `save-${Date.now()}`);
  const previous = [...(events.runs || [])].reverse().find((run) => run.id === id) || {};
  return {
    ...previous,
    id,
    sequence: Number.isFinite(Number(detail.sequence)) ? Number(detail.sequence) : previous.sequence || 0,
    status: String(detail.status || previous.status || "running"),
    label: String(detail.label || previous.label || "Saved files"),
    message: String(detail.message || previous.message || ""),
    changedFiles: Array.isArray(detail.changedFiles) && detail.changedFiles.length > 0
      ? detail.changedFiles.map(String)
      : previous.changedFiles || [],
    startedAt: String(detail.startedAt || previous.startedAt || new Date().toISOString()),
    finishedAt: String(detail.finishedAt || previous.finishedAt || ""),
    durationMs: Number.isFinite(Number(detail.durationMs)) ? Number(detail.durationMs) : previous.durationMs || 0,
    error: String(detail.error || previous.error || ""),
    reload: Boolean(detail.reload),
  };
}

function upsertRun(events, run) {
  const index = events.runs.findIndex((candidate) => candidate.id === run.id);
  if (index >= 0) {
    events.runs[index] = run;
  } else {
    events.runs.push(run);
  }
  if (events.runs.length > MAX_RUNS) {
    events.runs = events.runs.slice(events.runs.length - MAX_RUNS);
  }
}

function pushEntry(events, kind, entry) {
  if (!Array.isArray(events[kind])) {
    events[kind] = [];
  }
  events[kind].push(entry);
  if (events[kind].length > MAX_ENTRIES_PER_KIND) {
    events[kind] = events[kind].slice(events[kind].length - MAX_ENTRIES_PER_KIND);
  }
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
        detail: { level, args: args.map(consoleDetailValue) },
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

function consoleDetailValue(value) {
  if (value instanceof Error) {
    return consoleValue(value);
  }
  try {
    JSON.stringify(value);
    return value;
  } catch (_err) {
    return consoleValue(value);
  }
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
