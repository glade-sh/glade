import { emitPageReference } from "./navigation-service.mjs";

const LAB_STORAGE_KEY = "glade:component-lab:v1";
const FORM_FACTORS = new Set(["Large", "Medium", "Small"]);
const FIT_MODES = new Set(["fit", "actual", "full"]);
const LAB_CONTEXT_KINDS = new Set([
  "component",
  "appPage",
  "homePage",
  "recordPage",
  "tab",
  "urlAddressable",
  "quickAction",
  "communityPage",
  "utilityBar",
  "flowScreen",
  "flowAction",
]);
const LAB_CONTEXT_APP_LABEL_BY_KIND = {
  component: "Component",
  appPage: "App",
  homePage: "Home page",
  tab: "Tab",
  urlAddressable: "URL state",
  quickAction: "Action",
  communityPage: "Community page",
  utilityBar: "Utility",
  flowScreen: "Flow API name",
  flowAction: "Flow API name",
};
const LAB_CONTEXT_LABEL_BY_KIND = {
  component: "Component",
  appPage: "App page",
  homePage: "Home page",
  recordPage: "Record page",
  tab: "Tab",
  urlAddressable: "URL addressable",
  quickAction: "Quick action",
  communityPage: "Community page",
  utilityBar: "Utility bar",
  flowScreen: "Flow screen",
  flowAction: "Flow action",
};
const CONTEXT_PROP_NAMES = new Set([
  "appName",
  "communitySite",
  "flowApiName",
  "formFactor",
  "objectApiName",
  "pageReference",
  "recordId",
  "state",
]);

export function bootComponentLab(root = document.body, config = {}) {
  bindHomeModeTabs(root);
  const lab = root.querySelector("[data-glade-component-lab]");
  if (!lab) {
    return null;
  }
  const model = readWorkbenchModel();
  const controls = {
    picker: lab.querySelector("[data-glade-lab-component-picker]"),
    search: lab.querySelector("[data-glade-lab-component-search]"),
    selected: lab.querySelector("[data-glade-lab-selected]"),
    routeLink: lab.querySelector("[data-glade-lab-route-link]"),
    host: lab.querySelector("[data-glade-lab-host]"),
    hostShell: lab.querySelector("[data-glade-lab-host-shell]"),
    propList: lab.querySelector("[data-glade-lab-prop-list]"),
    resetProps: lab.querySelector("[data-glade-lab-reset-props]"),
    componentList: lab.querySelector("[data-glade-lab-component-list]"),
    componentRail: lab.querySelector("[data-glade-lab-components-rail]"),
    propsRail: lab.querySelector("[data-glade-lab-props-rail]"),
    contextPanel: lab.querySelector("[data-glade-lab-context]"),
    contextKind: lab.querySelector("[data-glade-lab-context-kind]"),
    contextObject: lab.querySelector("[data-glade-lab-context-object]"),
    contextObjectResults: lab.querySelector("[data-glade-lab-context-object-results]"),
    contextRecord: lab.querySelector("[data-glade-lab-context-record]"),
    contextRecordResults: lab.querySelector("[data-glade-lab-context-record-results]"),
    contextSampleRecord: lab.querySelector("[data-glade-lab-context-sample-record]"),
    contextApp: lab.querySelector("[data-glade-lab-context-app]"),
    contextAppLabel: lab.querySelector("[data-glade-lab-context-app-label]"),
    contextStateKey: lab.querySelector("[data-glade-lab-context-state-key]"),
    contextStateValue: lab.querySelector("[data-glade-lab-context-state-value]"),
    contextSummary: lab.querySelector("[data-glade-lab-context-summary]"),
    contextStrip: lab.querySelector("[data-glade-lab-context-strip]"),
    contextStripParts: Object.fromEntries(
      Array.from(lab.querySelectorAll("[data-glade-lab-strip-part]")).map((part) => [
        part.dataset.gladeLabStripPart,
        part,
      ]),
    ),
    contextGroups: Array.from(lab.querySelectorAll("[data-glade-lab-context-group]")),
    viewOptions: Array.from(lab.querySelectorAll("[data-glade-lab-view-option]")),
    formFactorOptions: Array.from(lab.querySelectorAll("[data-glade-lab-form-factor-option]")),
    fitOptions: Array.from(lab.querySelectorAll("[data-glade-lab-fit-option]")),
    options: Array.from(lab.querySelectorAll("[data-glade-lab-component-option]")),
  };
  const saved = readLabState();
  const state = {
    selected: selectedComponentName(model, saved.selected),
    formFactor: normalizeFormFactor(saved.formFactor),
    fitMode: normalizeLabFitMode(saved.fitMode),
    propsByComponent: saved.propsByComponent || {},
    componentStateByComponent: saved.componentStateByComponent && typeof saved.componentStateByComponent === "object"
      ? saved.componentStateByComponent
      : {},
    mounted: null,
    mountedComponent: "",
    objectSearchRequest: 0,
    recordSearchRequest: 0,
    contextObjectSearchRequest: 0,
    contextRecordSearchRequest: 0,
    viewMode: normalizeLabViewMode(saved.viewMode),
    context: normalizeLabContext(saved.context, model),
  };
  restoreSelectedComponentState(model, state, state.selected, saved);
  applyLabContextToControls(state, controls);
  if (controls.picker && state.selected) {
    controls.picker.value = state.selected;
  }
  const render = () => renderLab(lab, model, state, controls, config);
  lab.addEventListener("click", (event) => {
    const viewOption = event.target.closest("[data-glade-lab-view-option]");
    if (viewOption) {
      event.preventDefault();
      state.viewMode = normalizeLabViewMode(viewOption.dataset.gladeLabViewOption);
      applyLabViewState(lab, state, controls);
      persistLabState(state);
      return;
    }
    const fitOption = event.target.closest("[data-glade-lab-fit-option]");
    if (fitOption) {
      event.preventDefault();
      state.fitMode = normalizeLabFitMode(fitOption.dataset.gladeLabFitOption);
      renderFitControls(state, controls);
      applyHostViewport(state, controls);
      persistLabState(state);
      return;
    }
    const stripPart = event.target.closest("[data-glade-lab-strip-part]");
    if (stripPart) {
      event.preventDefault();
      const target = focusLabStripPart(stripPart.dataset.gladeLabStripPart, controls);
      closeLabSearchResultsExcept(controls, target);
      return;
    }
    const option = event.target.closest("[data-glade-lab-component-option]");
    if (option) {
      event.preventDefault();
      selectComponent(model, state, controls, option.dataset.gladeComponent || "");
      render();
      return;
    }
    const formFactor = event.target.closest("[data-glade-lab-form-factor-option]");
    if (formFactor) {
      event.preventDefault();
      selectFormFactor(model, state, formFactor.dataset.gladeLabFormFactorOption);
      render();
      return;
    }
    const reset = event.target.closest("[data-glade-lab-reset-props]");
    if (reset) {
      event.preventDefault();
      resetComponentProps(state);
      render();
      return;
    }
    if (event.target.closest("[data-glade-lab-context-sample-record]")) {
      event.preventDefault();
      state.context.recordId = model.sampleRecordId || "001000000000001AAA";
      applyLabContextToControls(state, controls);
      clearSearchResults(controls.contextRecordResults, controls.contextRecord);
      syncContextProperties(model, state);
      syncPropertyInputs(model, state, controls);
      applyFocusedProps(model, state, controls, config);
      persistLabState(state);
      return;
    }
    const contextObjectResult = event.target.closest("[data-glade-lab-context-object-result]");
    if (contextObjectResult) {
      event.preventDefault();
      state.context.objectApiName = contextObjectResult.dataset.gladeApiName || "";
      state.context.recordId = "";
      applyLabContextToControls(state, controls);
      clearSearchResults(controls.contextObjectResults, controls.contextObject);
      clearSearchResults(controls.contextRecordResults, controls.contextRecord);
      syncContextProperties(model, state);
      syncPropertyInputs(model, state, controls);
      applyFocusedProps(model, state, controls, config);
      persistLabState(state);
      refreshLabContextRecordSearch(state, controls);
      return;
    }
    const contextRecordResult = event.target.closest("[data-glade-lab-context-record-result]");
    if (contextRecordResult) {
      event.preventDefault();
      state.context.recordId = contextRecordResult.dataset.gladeRecordId || "";
      applyLabContextToControls(state, controls);
      clearSearchResults(controls.contextRecordResults, controls.contextRecord);
      syncContextProperties(model, state);
      syncPropertyInputs(model, state, controls);
      applyFocusedProps(model, state, controls, config);
      persistLabState(state);
      return;
    }
    if (!event.target.closest("[data-glade-combobox-shell]")) {
      closeSearchResults(controls.contextObjectResults, controls.contextObject);
      closeSearchResults(controls.contextRecordResults, controls.contextRecord);
    }
  });
  controls.propList?.addEventListener("click", (event) => {
    const objectResult = event.target.closest("[data-glade-lab-object-result]");
    if (objectResult) {
      event.preventDefault();
      setPropertyValue(model, state, controls, config, "objectApiName", objectResult.dataset.gladeApiName || "");
      const input = controls.propList.querySelector('[data-glade-lab-prop="objectApiName"]');
      const results = controls.propList.querySelector("[data-glade-lab-object-results]");
      closeSearchResults(results, input);
      refreshLabRecordSearch(model, state, controls);
      return;
    }
    const recordResult = event.target.closest("[data-glade-lab-record-result]");
    if (recordResult) {
      event.preventDefault();
      setPropertyValue(model, state, controls, config, "recordId", recordResult.dataset.gladeRecordId || "");
      const input = controls.propList.querySelector('[data-glade-lab-prop="recordId"]');
      const results = controls.propList.querySelector("[data-glade-lab-record-results]");
      closeSearchResults(results, input);
    }
  });
  controls.picker?.addEventListener("change", () => {
    selectComponent(model, state, controls, controls.picker.value);
    render();
  });
  controls.search?.addEventListener("input", () => renderComponentOptions(model, state, controls));
  controls.propList?.addEventListener("input", (event) => {
    const input = event.target.closest("[data-glade-lab-prop]");
    if (!input) {
      return;
    }
    const isObjectInput = input.dataset.gladeLabObjectInput !== undefined;
    const objectCleared = isObjectInput && !input.value.trim();
    const values = componentPropValues(model, state);
    values[input.dataset.gladeLabProp] = controlValue(input);
    if (objectCleared) {
      values.recordId = "";
    }
    state.propsByComponent[state.selected] = values;
    persistLabState(state);
    applyFocusedProps(model, state, controls, config);
    if (isObjectInput) {
      refreshLabObjectSearch(state, controls);
      if (objectCleared) {
        state.recordSearchRequest += 1;
        const recordInput = controls.propList.querySelector('[data-glade-lab-prop="recordId"]');
        const recordResults = controls.propList.querySelector("[data-glade-lab-record-results]");
        setControlValue(recordInput, "");
        clearSearchResults(recordResults, recordInput);
        return;
      }
      refreshLabRecordSearch(model, state, controls);
    } else if (input.dataset.gladeLabRecordInput !== undefined) {
      refreshLabRecordSearch(model, state, controls);
    }
  });
  const updateContextFromControls = (event) => {
    readLabContextFromControls(state, controls);
    const clearedContextObject = event?.target === controls.contextObject && !controls.contextObject.value.trim();
    if (clearedContextObject) {
      state.context.recordId = "";
      setControlValue(controls.contextRecord, "");
      state.contextRecordSearchRequest += 1;
      clearSearchResults(controls.contextRecordResults, controls.contextRecord);
    }
    if (event?.target === controls.contextKind) {
      closeSearchResults(controls.contextObjectResults, controls.contextObject);
      closeSearchResults(controls.contextRecordResults, controls.contextRecord);
    }
    syncContextProperties(model, state);
    syncPropertyInputs(model, state, controls);
    renderLabContextControls(model, state, controls);
    persistLabState(state);
    applyFocusedProps(model, state, controls, config);
    if (event?.target === controls.contextObject) {
      refreshLabContextObjectSearch(state, controls);
      if (!clearedContextObject) {
        refreshLabContextRecordSearch(state, controls);
      }
    } else if (event?.target === controls.contextRecord) {
      refreshLabContextRecordSearch(state, controls);
    }
  };
  for (const control of [
    controls.contextKind,
    controls.contextObject,
    controls.contextRecord,
    controls.contextApp,
    controls.contextStateKey,
    controls.contextStateValue,
  ]) {
    if (control) {
      control.addEventListener("input", updateContextFromControls);
      control.addEventListener("change", updateContextFromControls);
    }
  }
  controls.contextObject?.addEventListener("focus", () => refreshLabContextObjectSearch(state, controls));
  controls.contextRecord?.addEventListener("focus", () => refreshLabContextRecordSearch(state, controls));
  root.addEventListener("glade:home-mode-changed", () => applyLabViewState(lab, state, controls));
  render();
  return { model, state, render };
}

function bindHomeModeTabs(root) {
  const tablist = root.querySelector("[data-glade-home-mode-tabs]");
  if (!tablist || tablist.__gladeHomeModeTabs) {
    return;
  }
  const tabs = Array.from(tablist.querySelectorAll("[data-glade-home-mode-tab]"));
  const panels = Array.from(root.querySelectorAll("[data-glade-home-panel]"));
  const select = (mode) => {
    const selectedMode = mode === "workbench" ? "workbench" : "lab";
    for (const tab of tabs) {
      const selected = tab.dataset.gladeHomeModeTab === selectedMode;
      tab.setAttribute("aria-selected", String(selected));
      tab.tabIndex = selected ? 0 : -1;
    }
    for (const panel of panels) {
      panel.hidden = panel.dataset.gladeHomePanel !== selectedMode;
    }
    root.dispatchEvent(new CustomEvent("glade:home-mode-changed", { detail: { mode: selectedMode } }));
  };
  tablist.addEventListener("click", (event) => {
    const tab = event.target.closest("[data-glade-home-mode-tab]");
    if (!tab) {
      return;
    }
    event.preventDefault();
    select(tab.dataset.gladeHomeModeTab || "lab");
  });
  select(tabs.find((tab) => tab.getAttribute("aria-selected") === "true")?.dataset.gladeHomeModeTab || "lab");
  tablist.__gladeHomeModeTabs = true;
}

function renderLab(_lab, model, state, controls, config) {
  const component = findComponent(model, state.selected);
  if (!component) {
    applyLabViewState(_lab, state, controls);
    renderComponentOptions(model, state, controls);
    renderLabEmptyState(controls, "No component selected", "Choose an exposed Lightning Web Component from Setup to start previewing.");
    renderLabContextStrip(model, state, controls, null);
    persistLabState(state);
    return;
  }
  applyLabViewState(_lab, state, controls);
  if (controls.picker && controls.picker.value !== component.qualifiedName) {
    controls.picker.value = component.qualifiedName;
  }
  if (controls.selected) {
    controls.selected.textContent = component.qualifiedName;
  }
  if (controls.routeLink) {
    controls.routeLink.setAttribute("href", componentRoute(component));
  }
  renderFormFactorControls(state, controls);
  renderFitControls(state, controls);
  renderComponentOptions(model, state, controls);
  renderLabContextControls(model, state, controls);
  renderLabContextStrip(model, state, controls, component);
  syncContextProperties(model, state);
  renderPropertyControls(model, state, controls);
  mountFocusedComponent(model, state, controls, config);
  persistLabState(state);
}

function applyLabViewState(lab, state, controls) {
  if (!lab) {
    return;
  }
  state.viewMode = normalizeLabViewMode(state.viewMode);
  lab.dataset.gladeLabView = state.viewMode;
  for (const option of controls.viewOptions || []) {
    option.setAttribute("aria-pressed", String(option.dataset.gladeLabViewOption === state.viewMode));
  }
  const workbench = lab.closest("[data-glade-workbench-console]");
  if (workbench) {
    const labPanel = lab.closest('[data-glade-home-panel="lab"]');
    workbench.dataset.gladeLabView = !labPanel || !labPanel.hidden ? state.viewMode : "setup";
  }
}

function renderComponentOptions(model, state, controls) {
  const query = normalize(controls.search?.value || "");
  let visibleCount = 0;
  for (const option of controls.options || []) {
    const component = findComponent(model, option.dataset.gladeComponent);
    const matched = !query || componentMatches(component, query);
    option.hidden = !matched;
    option.dataset.gladeSelected = String(component?.qualifiedName === state.selected);
    if (matched) {
      visibleCount += 1;
    }
  }
  const list = controls.componentList || controls.options?.[0]?.closest("[data-glade-lab-component-list]");
  let empty = list?.querySelector("[data-glade-lab-component-empty]");
  if (list && !empty) {
    empty = document.createElement("p");
    empty.className = "glade-lab-empty";
    empty.dataset.gladeLabComponentEmpty = "";
    list.append(empty);
  }
  if (empty) {
    empty.textContent = query ? "No components match this filter." : "No exposed components found.";
    empty.hidden = visibleCount !== 0;
  }
}

function renderPropertyControls(model, state, controls) {
  if (!controls.propList) {
    return;
  }
  const component = findComponent(model, state.selected);
  const props = componentProperties(component).filter((prop) => !isContextProperty(prop.name));
  const values = componentPropValues(model, state);
  if (controls.resetProps) {
    controls.resetProps.disabled = props.length === 0;
  }
  controls.propList.replaceChildren();
  if (props.length === 0) {
    const empty = document.createElement("p");
    empty.className = "glade-lab-empty";
    empty.textContent = "No public properties found.";
    controls.propList.append(empty);
    return;
  }
  for (const prop of props) {
    controls.propList.append(renderPropertyControl(prop, values[prop.name] ?? ""));
  }
}

function renderLabContextStrip(model, state, controls, component) {
  if (!controls.contextStrip) {
    return;
  }
  const context = normalizeLabContext(state.context, model);
  const statePairs = currentLabStatePairs(state);
  const hasState = Object.keys(statePairs).length > 0;
  setStripPart(controls.contextStripParts?.component, component?.qualifiedName || "No component", Boolean(component));
  setStripPart(controls.contextStripParts?.context, LAB_CONTEXT_LABEL_BY_KIND[context.kind] || "Context", true);
  setStripPart(controls.contextStripParts?.object, context.objectApiName || "No object", labContextUsesRecord(context.kind));
  setStripPart(controls.contextStripParts?.record, context.recordId || model.sampleRecordId || "No record", labContextUsesRecord(context.kind));
  setStripPart(controls.contextStripParts?.formFactor, state.formFactor || "Large", true);
  setStripPart(controls.contextStripParts?.state, hasState ? `${Object.keys(statePairs)[0]}=${Object.values(statePairs)[0]}` : "", hasState);
}

function setStripPart(button, text, visible) {
  if (!button) {
    return;
  }
  button.hidden = !visible;
  button.textContent = text || "";
}

function focusLabStripPart(part, controls) {
  const target = part === "component"
    ? controls.search
    : part === "context"
      ? controls.contextKind
      : part === "object"
        ? controls.contextObject
        : part === "record"
          ? controls.contextRecord
          : part === "state"
            ? controls.contextStateKey
            : controls.formFactorOptions?.find((option) => option.getAttribute("aria-pressed") === "true")
              || controls.formFactorOptions?.[0];
  target?.focus?.();
  return target || null;
}

function closeLabSearchResultsExcept(controls, exceptInput) {
  const searches = [
    [controls.contextObjectResults, controls.contextObject],
    [controls.contextRecordResults, controls.contextRecord],
    [
      controls.propList?.querySelector("[data-glade-lab-object-results]"),
      controls.propList?.querySelector('[data-glade-lab-prop="objectApiName"]'),
    ],
    [
      controls.propList?.querySelector("[data-glade-lab-record-results]"),
      controls.propList?.querySelector('[data-glade-lab-prop="recordId"]'),
    ],
  ];
  for (const [results, input] of searches) {
    if (input && input === exceptInput) {
      continue;
    }
    closeSearchResults(results, input);
  }
}

function renderLabContextControls(model, state, controls) {
  if (!controls.contextPanel) {
    return;
  }
  applyLabContextToControls(state, controls);
  const recordEnabled = labContextUsesRecord(state.context.kind);
  const appEnabled = labContextUsesApp(state.context.kind);
  for (const group of controls.contextGroups || []) {
    const name = group.dataset.gladeLabContextGroup;
    const visible = name === "record" ? recordEnabled : name === "app" ? appEnabled : true;
    group.hidden = !visible;
    for (const input of group.querySelectorAll("input, select, textarea, button")) {
      input.disabled = !visible;
    }
  }
  if (controls.contextAppLabel) {
    controls.contextAppLabel.textContent = LAB_CONTEXT_APP_LABEL_BY_KIND[state.context.kind] || "App / page";
  }
  if (controls.contextSummary) {
    controls.contextSummary.textContent = labContextSummary(model, state);
  }
  if (!recordEnabled) {
    clearSearchResults(controls.contextObjectResults, controls.contextObject);
    clearSearchResults(controls.contextRecordResults, controls.contextRecord);
  }
}

function readLabContextFromControls(state, controls) {
  state.context = normalizeLabContext({
    ...state.context,
    kind: controls.contextKind?.value || state.context?.kind,
    objectApiName: controls.contextObject?.value || "",
    recordId: controls.contextRecord?.value || "",
    appName: controls.contextApp?.value || "",
    stateKey: controls.contextStateKey?.value || "",
    stateValue: controls.contextStateValue?.value || "",
  });
}

function applyLabContextToControls(state, controls) {
  const context = normalizeLabContext(state.context);
  state.context = context;
  setControlValue(controls.contextKind, context.kind);
  setControlValue(controls.contextObject, context.objectApiName);
  setControlValue(controls.contextRecord, context.recordId);
  setControlValue(controls.contextApp, context.appName);
  setControlValue(controls.contextStateKey, context.stateKey);
  setControlValue(controls.contextStateValue, context.stateValue);
}

function syncContextProperties(model, state) {
  const component = findComponent(model, state.selected);
  if (!component) {
    return;
  }
  const values = state.propsByComponent[state.selected] || {};
  const recordEnabled = labContextUsesRecord(state.context.kind);
  const appEnabled = labContextUsesApp(state.context.kind);
  for (const prop of componentProperties(component)) {
    switch (prop.name) {
      case "recordId":
        values.recordId = recordEnabled
          ? state.context.recordId || ""
          : state.context.kind === "component"
            ? values.recordId || state.context.recordId || model.sampleRecordId || ""
            : "";
        break;
      case "objectApiName":
        values.objectApiName = recordEnabled
          ? state.context.objectApiName || ""
          : state.context.kind === "component"
            ? values.objectApiName || state.context.objectApiName || "Account"
            : "";
        break;
      case "formFactor":
        values.formFactor = state.formFactor || "Large";
        break;
      case "appName":
        values.appName = appEnabled ? state.context.appName || "" : "";
        break;
      case "communitySite":
        values.communitySite = state.context.kind === "communityPage" ? state.context.appName || "" : "";
        break;
      case "flowApiName":
        values.flowApiName = labContextUsesFlow(state.context.kind) ? state.context.appName || "" : "";
        break;
      case "state":
        values.state = currentLabStatePairs(state);
        break;
      case "pageReference":
        values.pageReference = currentLabPageReference(component, state);
        break;
      default:
        break;
    }
  }
  state.propsByComponent[state.selected] = values;
}

function syncPropertyInputs(model, state, controls) {
  if (!controls.propList) {
    return;
  }
  const values = componentPropValues(model, state);
  for (const prop of componentProperties(findComponent(model, state.selected))) {
    const input = controls.propList.querySelector(`[data-glade-lab-prop="${prop.name}"]`);
    if (!input) {
      continue;
    }
    const value = values[prop.name] ?? "";
    if (input.type === "checkbox") {
      input.checked = value === true || value === "true";
    } else if (input.value !== String(value)) {
      input.value = value;
    }
  }
}

function renderPropertyControl(prop, value) {
  const label = document.createElement("label");
  label.className = "glade-lab-prop-control";
  const caption = document.createElement("span");
  caption.textContent = prop.label || prop.name;
  if (prop.name === "objectApiName" || prop.name === "recordId") {
    label.append(caption, renderSearchPropertyControl(prop, value));
    return label;
  }
  const input = document.createElement("input");
  input.dataset.gladeLabProp = prop.name;
  input.autocomplete = "off";
  if (isBooleanProperty(prop)) {
    input.type = "checkbox";
    input.checked = value === true || value === "true";
  } else if (isNumberProperty(prop)) {
    input.type = "number";
    input.value = value;
  } else {
    input.type = "text";
    input.value = value;
  }
  label.append(caption, input);
  return label;
}

function renderSearchPropertyControl(prop, value) {
  const shell = document.createElement("span");
  shell.className = "glade-search-field glade-lab-search-field";
  const input = document.createElement("input");
  input.type = "search";
  input.value = value;
  input.dataset.gladeLabProp = prop.name;
  input.autocomplete = "off";
  input.setAttribute("role", "combobox");
  input.setAttribute("aria-autocomplete", "list");
  input.setAttribute("aria-expanded", "false");
  input.setAttribute("aria-haspopup", "listbox");
  const results = document.createElement("div");
  results.className = "glade-search-results glade-lab-search-results";
  results.hidden = true;
  results.setAttribute("role", "listbox");
  if (prop.name === "objectApiName") {
    input.dataset.gladeLabObjectInput = "";
    results.dataset.gladeLabObjectResults = "";
    results.id = "glade-lab-object-results";
  } else {
    input.dataset.gladeLabRecordInput = "";
    results.dataset.gladeLabRecordResults = "";
    results.id = "glade-lab-record-results";
  }
  input.setAttribute("aria-controls", results.id);
  shell.append(input, results);
  return shell;
}

function mountFocusedComponent(model, state, controls, config) {
  const component = findComponent(model, state.selected);
  const host = controls.host;
  if (!component || !host) {
    return;
  }
  const attrs = focusedComponentAttrs(model, state);
  applyHostViewport(state, controls);
  updateContextScripts(component, attrs, config, state);
  host.replaceChildren();
  if (!window.$Lightning || typeof window.$Lightning.createComponent !== "function") {
    renderMountError(host, "Runtime unavailable", "Local LWC runtime is not ready.");
    return;
  }
  window.$Lightning.createComponent(component.qualifiedName, attrs, host.id, (_cmp, status, message) => {
    if (status === "SUCCESS") {
      state.mounted = _cmp || null;
      state.mountedComponent = component.qualifiedName;
      return;
    }
    state.mounted = null;
    state.mountedComponent = "";
    renderMountError(host, `Unable to mount ${component.qualifiedName}`, message || "The local runtime did not return a successful mount status.");
  });
}

function renderLabEmptyState(controls, title, message) {
  if (!controls.host) {
    return;
  }
  controls.host.replaceChildren();
  const empty = document.createElement("section");
  empty.className = "glade-lab-empty-state";
  empty.dataset.gladeLabEmptyState = "";
  empty.innerHTML = `<strong></strong><p></p>`;
  empty.querySelector("strong").textContent = title;
  empty.querySelector("p").textContent = message;
  controls.host.append(empty);
}

function renderMountError(host, title, message) {
  host.replaceChildren();
  const panel = document.createElement("section");
  panel.className = "glade-mount-error";
  panel.dataset.gladeMountError = "";
  panel.innerHTML = `<strong></strong><p></p>`;
  panel.querySelector("strong").textContent = title;
  panel.querySelector("p").textContent = message;
  host.append(panel);
}

function applyFocusedProps(model, state, controls, config) {
  const component = findComponent(model, state.selected);
  if (!component) {
    return;
  }
  const attrs = focusedComponentAttrs(model, state);
  applyHostViewport(state, controls);
  updateContextScripts(component, attrs, config, state);
  if (!state.mounted || state.mountedComponent !== component.qualifiedName || !controls.host?.contains(state.mounted)) {
    mountFocusedComponent(model, state, controls, config);
    return;
  }
  for (const [name, value] of Object.entries(attrs)) {
    state.mounted[name] = value;
  }
}

function focusedComponentAttrs(model, state) {
  const component = findComponent(model, state.selected);
  return {
    ...componentPropValues(model, state),
    ...labContextAttrs(component, state),
  };
}

function updateContextScripts(component, attrs, config, state) {
  const context = currentLabContext(component, attrs, state);
  const pageReference = currentLabPageReference(component, state);
  const contextNode = document.getElementById("glade-lwc-context");
  if (contextNode) {
    contextNode.textContent = JSON.stringify(context);
  }
  const configNode = document.getElementById("glade-lightning-config");
  if (configNode) {
    configNode.textContent = JSON.stringify({ ...config, pageReference });
  }
  emitPageReference(pageReference);
  document.dispatchEvent(new CustomEvent("glade:context-changed", { detail: { context, pageReference } }));
}

function componentPropValues(model, state) {
  const component = findComponent(model, state.selected);
  const existing = state.propsByComponent[state.selected] || {};
  const values = {};
  for (const prop of componentProperties(component)) {
    values[prop.name] = existing[prop.name] !== undefined ? existing[prop.name] : defaultPropertyValue(prop, model, state);
  }
  state.propsByComponent[state.selected] = values;
  return values;
}

function componentProperties(component) {
  const seen = new Set();
  const props = [];
  for (const prop of component?.apiProperties || []) {
    const name = String(prop.name || "").trim();
    const key = name.toLowerCase();
    if (!name || seen.has(key)) {
      continue;
    }
    seen.add(key);
    props.push(prop);
  }
  props.sort((left, right) => {
    if (left.name === "objectApiName" && right.name === "recordId") {
      return -1;
    }
    if (left.name === "recordId" && right.name === "objectApiName") {
      return 1;
    }
    return 0;
  });
  return props;
}

function defaultPropertyValue(prop, model, state) {
  if (prop.default !== undefined && prop.default !== null && prop.default !== "") {
    return prop.default;
  }
  if (prop.name === "recordId") {
    return labContextUsesRecord(state.context?.kind) || state.context?.kind === "component"
      ? state.context.recordId || model.sampleRecordId || ""
      : "";
  }
  if (prop.name === "objectApiName") {
    return labContextUsesRecord(state.context?.kind) || state.context?.kind === "component"
      ? state.context.objectApiName || "Account"
      : "";
  }
  if (prop.name === "formFactor") {
    return state.formFactor || "Large";
  }
  if (prop.name === "appName") {
    return labContextUsesApp(state.context?.kind) ? state.context.appName || "" : "";
  }
  if (prop.name === "flowApiName") {
    return labContextUsesFlow(state.context?.kind) ? state.context.appName || "" : "";
  }
  if (prop.name === "communitySite") {
    return state.context?.kind === "communityPage" ? state.context.appName || "" : "";
  }
  if (isBooleanProperty(prop)) {
    return false;
  }
  return "";
}

function renderFormFactorControls(state, controls) {
  applyHostViewport(state, controls);
  for (const option of controls.formFactorOptions || []) {
    option.setAttribute("aria-pressed", String(option.dataset.gladeLabFormFactorOption === state.formFactor));
  }
}

function renderFitControls(state, controls) {
  state.fitMode = normalizeLabFitMode(state.fitMode);
  for (const option of controls.fitOptions || []) {
    option.setAttribute("aria-pressed", String(option.dataset.gladeLabFitOption === state.fitMode));
  }
}

function applyHostViewport(state, controls) {
  const factor = normalizeFormFactor(state.formFactor);
  state.formFactor = factor;
  state.fitMode = normalizeLabFitMode(state.fitMode);
  if (controls.hostShell) {
    controls.hostShell.dataset.gladeFormFactor = factor;
    controls.hostShell.dataset.gladeFitMode = state.fitMode;
  }
  if (controls.host) {
    controls.host.dataset.gladeFormFactor = factor;
    controls.host.dataset.gladeFitMode = state.fitMode;
  }
}

function selectFormFactor(model, state, value) {
  const formFactor = normalizeFormFactor(value);
  state.formFactor = formFactor;
  const component = findComponent(model, state.selected);
  if (componentProperties(component).some((prop) => prop.name === "formFactor")) {
    const values = componentPropValues(model, state);
    values.formFactor = formFactor;
    state.propsByComponent[state.selected] = values;
  }
}

function setPropertyValue(model, state, controls, config, name, value) {
  const input = controls.propList?.querySelector(`[data-glade-lab-prop="${name}"]`);
  if (input) {
    if (input.type === "checkbox") {
      input.checked = Boolean(value);
    } else {
      input.value = value;
    }
  }
  const values = componentPropValues(model, state);
  values[name] = value;
  state.propsByComponent[state.selected] = values;
  persistLabState(state);
  applyFocusedProps(model, state, controls, config);
}

async function refreshLabObjectSearch(state, controls) {
  const input = controls.propList?.querySelector("[data-glade-lab-object-input]");
  const results = controls.propList?.querySelector("[data-glade-lab-object-results]");
  if (!input || !results) {
    return;
  }
  const request = ++state.objectSearchRequest;
  try {
    const params = new URLSearchParams({ q: input.value || "" });
    const response = await fetch(`/lightning/local/objects.json?${params.toString()}`, {
      headers: { Accept: "application/json" },
    });
    if (!response.ok || request !== state.objectSearchRequest) {
      return;
    }
    const payload = await response.json();
    if (request !== state.objectSearchRequest) {
      return;
    }
    renderLabObjectResults(results, input, payload.objects || []);
  } catch (_err) {
    clearSearchResults(results, input);
  }
}

async function refreshLabRecordSearch(model, state, controls) {
  const input = controls.propList?.querySelector("[data-glade-lab-record-input]");
  const results = controls.propList?.querySelector("[data-glade-lab-record-results]");
  if (!input || !results) {
    return;
  }
  const objectName = currentObjectApiName(model, state, controls);
  if (!objectName) {
    clearSearchResults(results, input);
    return;
  }
  const request = ++state.recordSearchRequest;
  try {
    const params = new URLSearchParams({ object: objectName, q: input.value || "" });
    const response = await fetch(`/lightning/local/records.json?${params.toString()}`, {
      headers: { Accept: "application/json" },
    });
    if (!response.ok || request !== state.recordSearchRequest) {
      return;
    }
    const payload = await response.json();
    if (request !== state.recordSearchRequest) {
      return;
    }
    renderLabRecordResults(results, input, payload.records || []);
  } catch (_err) {
    clearSearchResults(results, input);
  }
}

async function refreshLabContextObjectSearch(state, controls) {
  const input = controls.contextObject;
  const results = controls.contextObjectResults;
  if (!input || !results || !labContextUsesRecord(state.context.kind)) {
    clearSearchResults(results, input);
    return;
  }
  const request = ++state.contextObjectSearchRequest;
  try {
    const params = new URLSearchParams({ q: input.value || "" });
    const response = await fetch(`/lightning/local/objects.json?${params.toString()}`, {
      headers: { Accept: "application/json" },
    });
    if (!response.ok || request !== state.contextObjectSearchRequest) {
      return;
    }
    const payload = await response.json();
    if (request !== state.contextObjectSearchRequest) {
      return;
    }
    renderLabContextObjectResults(results, input, payload.objects || []);
  } catch (_err) {
    clearSearchResults(results, input);
  }
}

async function refreshLabContextRecordSearch(state, controls) {
  const input = controls.contextRecord;
  const results = controls.contextRecordResults;
  if (!input || !results || !labContextUsesRecord(state.context.kind)) {
    clearSearchResults(results, input);
    return;
  }
  const objectName = state.context.objectApiName || controls.contextObject?.value?.trim();
  if (!objectName) {
    clearSearchResults(results, input);
    return;
  }
  const request = ++state.contextRecordSearchRequest;
  try {
    const params = new URLSearchParams({ object: objectName, q: input.value || "" });
    const response = await fetch(`/lightning/local/records.json?${params.toString()}`, {
      headers: { Accept: "application/json" },
    });
    if (!response.ok || request !== state.contextRecordSearchRequest) {
      return;
    }
    const payload = await response.json();
    if (request !== state.contextRecordSearchRequest) {
      return;
    }
    renderLabContextRecordResults(results, input, payload.records || []);
  } catch (_err) {
    clearSearchResults(results, input);
  }
}

function renderLabObjectResults(results, input, objects) {
  clearSearchResults(results, input);
  if (!objects.length) {
    return;
  }
  openSearchResults(results, input);
  for (const [index, object] of objects.slice(0, 25).entries()) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "glade-search-result";
    button.dataset.gladeLabObjectResult = "";
    button.dataset.gladeApiName = object.apiName || "";
    button.id = `glade-lab-object-result-${index}`;
    button.setAttribute("role", "option");
    button.setAttribute("aria-selected", "false");
    button.innerHTML = `<strong></strong><span></span>`;
    button.querySelector("strong").textContent = object.label || object.apiName || "Object";
    button.querySelector("span").textContent = `${object.apiName || ""}${object.recordCount ? ` · ${object.recordCount} records` : ""}`;
    results.append(button);
  }
}

function renderLabContextObjectResults(results, input, objects) {
  clearSearchResults(results, input);
  if (!objects.length) {
    return;
  }
  openSearchResults(results, input);
  for (const [index, object] of objects.slice(0, 25).entries()) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "glade-search-result";
    button.dataset.gladeLabContextObjectResult = "";
    button.dataset.gladeApiName = object.apiName || "";
    button.id = `glade-lab-context-object-result-${index}`;
    button.setAttribute("role", "option");
    button.setAttribute("aria-selected", "false");
    button.innerHTML = `<strong></strong><span></span>`;
    button.querySelector("strong").textContent = object.label || object.apiName || "Object";
    button.querySelector("span").textContent = `${object.apiName || ""}${object.recordCount ? ` · ${object.recordCount} records` : ""}`;
    results.append(button);
  }
}

function renderLabContextRecordResults(results, input, records) {
  clearSearchResults(results, input);
  if (!records.length) {
    return;
  }
  openSearchResults(results, input);
  for (const [index, record] of records.slice(0, 25).entries()) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "glade-search-result";
    button.dataset.gladeLabContextRecordResult = "";
    button.dataset.gladeRecordId = record.id || "";
    button.id = `glade-lab-context-record-result-${index}`;
    button.setAttribute("role", "option");
    button.setAttribute("aria-selected", "false");
    button.innerHTML = `<strong></strong><span></span>`;
    button.querySelector("strong").textContent = record.title || record.id || "Record";
    button.querySelector("span").textContent = record.id || "";
    results.append(button);
  }
}

function renderLabRecordResults(results, input, records) {
  clearSearchResults(results, input);
  if (!records.length) {
    return;
  }
  openSearchResults(results, input);
  for (const [index, record] of records.slice(0, 25).entries()) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "glade-search-result";
    button.dataset.gladeLabRecordResult = "";
    button.dataset.gladeRecordId = record.id || "";
    button.id = `glade-lab-record-result-${index}`;
    button.setAttribute("role", "option");
    button.setAttribute("aria-selected", "false");
    button.innerHTML = `<strong></strong><span></span>`;
    button.querySelector("strong").textContent = record.title || record.id || "Record";
    button.querySelector("span").textContent = record.id || "";
    results.append(button);
  }
}

function clearSearchResults(results, input) {
  if (results) {
    results.replaceChildren();
  }
  closeSearchResults(results, input);
}

function openSearchResults(results, input) {
  if (results) {
    results.hidden = false;
  }
  if (input) {
    input.setAttribute("aria-expanded", "true");
  }
}

function closeSearchResults(results, input) {
  if (results) {
    results.hidden = true;
  }
  if (input) {
    input.setAttribute("aria-expanded", "false");
  }
}

function currentObjectApiName(model, state, controls) {
  const objectInput = controls.propList?.querySelector('[data-glade-lab-prop="objectApiName"]');
  if (objectInput) {
    return objectInput.value.trim();
  }
  const values = componentPropValues(model, state);
  return String(values.objectApiName || "Account").trim();
}

function currentLabContext(component, attrs, state) {
  const statePairs = currentLabStatePairs(state);
  const recordContext = labContextUsesRecord(state.context.kind);
  const appContext = labContextUsesApp(state.context.kind);
  const flowContext = labContextUsesFlow(state.context.kind);
  return {
    kind: state.context.kind,
    componentName: component.qualifiedName,
    recordId: recordContext ? String(state.context.recordId || attrs.recordId || "") : String(attrs.recordId || ""),
    objectApiName: recordContext ? String(state.context.objectApiName || attrs.objectApiName || "") : String(attrs.objectApiName || ""),
    appName: appContext ? String(state.context.appName || attrs.appName || "") : "",
    formFactor: String(attrs.formFactor || state.formFactor || "Large"),
    state: statePairs,
    community: {
      site: state.context.kind === "communityPage" ? String(state.context.appName || attrs.communitySite || "") : "",
    },
    flow: {
      apiName: flowContext ? String(state.context.appName || attrs.flowApiName || "") : "",
    },
  };
}

function currentLabPageReference(component, state) {
  const baseState = currentLabStatePairs(state);
  switch (state.context.kind) {
    case "recordPage":
      return {
        type: "standard__recordPage",
        attributes: {
          objectApiName: state.context.objectApiName || "",
          recordId: state.context.recordId || "",
          actionName: "view",
        },
        state: baseState,
      };
    case "tab":
      return {
        type: "standard__navItemPage",
        attributes: { apiName: state.context.appName || "Local" },
        state: baseState,
      };
    case "homePage":
      return {
        type: "standard__namedPage",
        attributes: { pageName: "home" },
        state: baseState,
      };
    case "urlAddressable":
      return {
        type: "standard__component",
        attributes: { componentName: component.qualifiedName },
        state: baseState,
      };
    case "quickAction":
      return {
        type: "standard__quickAction",
        attributes: {
          apiName: state.context.appName || "",
          objectApiName: state.context.objectApiName || "",
          recordId: state.context.recordId || "",
        },
        state: baseState,
      };
    case "communityPage":
      return {
        type: "comm__namedPage",
        attributes: { name: state.context.appName || "Home" },
        state: baseState,
      };
    case "utilityBar":
      return {
        type: "standard__component",
        attributes: { componentName: component.qualifiedName },
        state: baseState,
      };
    case "flowScreen":
    case "flowAction":
      return {
        type: "standard__flow",
        attributes: { flowApiName: state.context.appName || "" },
        state: baseState,
      };
    case "appPage":
      return {
        type: "standard__app",
        attributes: { appTarget: state.context.appName || "Local" },
        state: baseState,
      };
    default:
      return {
        type: "standard__component",
        attributes: { componentName: component.qualifiedName },
        state: baseState,
      };
  }
}

function labContextAttrs(component, state) {
  const propNames = new Set(componentProperties(component).map((prop) => prop.name));
  const attrs = {
    formFactor: state.formFactor || "Large",
  };
  if (labContextUsesRecord(state.context.kind)) {
    attrs.recordId = state.context.recordId || "";
    attrs.objectApiName = state.context.objectApiName || "";
  }
  if (labContextUsesApp(state.context.kind)) {
    attrs.appName = state.context.appName || "";
  }
  if (state.context.kind === "communityPage") {
    attrs.communitySite = state.context.appName || "";
  }
  if (labContextUsesFlow(state.context.kind)) {
    attrs.flowApiName = state.context.appName || "";
  }
  if (propNames.has("state")) {
    attrs.state = currentLabStatePairs(state);
  }
  if (propNames.has("pageReference")) {
    attrs.pageReference = currentLabPageReference(component, state);
  }
  return attrs;
}

function currentLabStatePairs(state) {
  const key = state.context.stateKey?.trim();
  if (!key) {
    return {};
  }
  return { [key]: state.context.stateValue || "" };
}

function labContextSummary(model, state) {
  if (labContextUsesRecord(state.context.kind)) {
    return `${state.context.objectApiName || "Object"} / ${state.context.recordId || model.sampleRecordId || "no record"}`;
  }
  if (labContextUsesFlow(state.context.kind)) {
    return state.context.appName ? `Flow ${state.context.appName}` : "Flow context";
  }
  if (state.context.kind === "homePage") {
    return "Home page context";
  }
  if (state.context.kind === "component") {
    return "Direct component preview";
  }
  return state.context.appName || LAB_CONTEXT_APP_LABEL_BY_KIND[state.context.kind] || "Page context";
}

function labContextUsesRecord(kind) {
  return kind === "recordPage" || kind === "quickAction";
}

function labContextUsesFlow(kind) {
  return kind === "flowScreen" || kind === "flowAction";
}

function labContextUsesApp(kind) {
  return kind !== "component" && kind !== "homePage";
}

function resetComponentProps(state) {
  if (state.selected) {
    delete state.propsByComponent[state.selected];
  }
  state.mounted = null;
  state.mountedComponent = "";
}

function selectedComponentName(model, saved) {
  if (findComponent(model, saved)) {
    return saved;
  }
  const active = model.active?.context?.componentName;
  if (findComponent(model, active)) {
    return active;
  }
  return (model.components || []).find((component) => component.exposed)?.qualifiedName || "";
}

function selectComponent(model, state, controls, qualifiedName) {
  if (!qualifiedName) {
    return;
  }
  if (state.selected !== qualifiedName) {
    captureSelectedComponentState(state);
    state.mounted = null;
    state.mountedComponent = "";
  }
  const previousViewMode = state.viewMode;
  state.selected = qualifiedName;
  restoreSelectedComponentState(model, state, qualifiedName, { viewMode: previousViewMode });
  applyLabContextToControls(state, controls);
  if (controls.picker) {
    controls.picker.value = qualifiedName;
  }
}

function captureSelectedComponentState(state) {
  if (!state.selected) {
    return;
  }
  state.componentStateByComponent[state.selected] = {
    formFactor: normalizeFormFactor(state.formFactor),
    fitMode: normalizeLabFitMode(state.fitMode),
    viewMode: normalizeLabViewMode(state.viewMode),
    context: normalizeLabContext(state.context),
  };
}

function restoreSelectedComponentState(model, state, qualifiedName, fallback = {}) {
  const component = findComponent(model, qualifiedName);
  const memory = state.componentStateByComponent[qualifiedName] || {};
  const defaults = defaultLabStateForComponent(component, model);
  state.formFactor = normalizeFormFactor(memory.formFactor || fallback.formFactor || defaults.formFactor);
  state.fitMode = normalizeLabFitMode(memory.fitMode || fallback.fitMode || defaults.fitMode);
  state.viewMode = normalizeLabViewMode(memory.viewMode || fallback.viewMode || defaults.viewMode);
  state.context = normalizeLabContext(memory.context || fallback.context || defaults.context, model);
}

function defaultLabStateForComponent(component, model = {}) {
  const recordContext = hasComponentProperty(component, "recordId") || hasComponentProperty(component, "objectApiName");
  return {
    formFactor: "Large",
    fitMode: "fit",
    viewMode: "preview",
    context: {
      kind: recordContext ? "recordPage" : "component",
      objectApiName: "Account",
      recordId: model.sampleRecordId || "001000000000001AAA",
      appName: "Local",
      stateKey: "",
      stateValue: "",
    },
  };
}

function hasComponentProperty(component, name) {
  return componentProperties(component).some((prop) => prop.name === name);
}

function componentRoute(component) {
  const [namespace, name] = String(component.qualifiedName || "").split(":");
  if (!namespace || !name) {
    return "/lwc";
  }
  return `/lwc/preview/component/${encodeURIComponent(namespace)}/${encodeURIComponent(name)}`;
}

function controlValue(input) {
  if (input.type === "checkbox") {
    return input.checked;
  }
  return input.value;
}

function isBooleanProperty(prop) {
  return normalize(prop?.type) === "boolean";
}

function isNumberProperty(prop) {
  return normalize(prop?.type) === "number";
}

function findComponent(model, qualifiedName) {
  const want = normalize(qualifiedName);
  return (model.components || []).find((component) => normalize(component.qualifiedName) === want);
}

function componentMatches(component, query) {
  const haystack = [
    component?.label,
    component?.name,
    component?.qualifiedName,
    ...(component?.targets || []),
  ].join(" ");
  return normalize(haystack).includes(query);
}

function readWorkbenchModel() {
  const node = document.getElementById("glade-lwc-workbench");
  if (!node) {
    return { components: [] };
  }
  try {
    return JSON.parse(node.textContent || "{}");
  } catch (_err) {
    return { components: [] };
  }
}

function readLabState() {
  if (typeof sessionStorage === "undefined") {
    return {};
  }
  try {
    const parsed = JSON.parse(sessionStorage.getItem(LAB_STORAGE_KEY) || "{}");
    return parsed && typeof parsed === "object" ? parsed : {};
  } catch (_err) {
    return {};
  }
}

function persistLabState(state) {
  if (typeof sessionStorage === "undefined") {
    return;
  }
  try {
    captureSelectedComponentState(state);
    sessionStorage.setItem(LAB_STORAGE_KEY, JSON.stringify({
      selected: state.selected,
      formFactor: state.formFactor,
      fitMode: normalizeLabFitMode(state.fitMode),
      propsByComponent: state.propsByComponent,
      componentStateByComponent: state.componentStateByComponent,
      viewMode: normalizeLabViewMode(state.viewMode),
      context: normalizeLabContext(state.context),
    }));
  } catch (_err) {
    // Component Lab persistence should never block preview work.
  }
}

function setControlValue(control, value) {
  if (!control || value === undefined || value === null) {
    return;
  }
  const text = String(value);
  if (control.tagName === "SELECT") {
    const hasOption = Array.from(control.options || []).some((option) => option.value === text);
    if (!hasOption) {
      return;
    }
  }
  control.value = text;
}

function normalizeLabContext(raw = {}, model = {}) {
  const context = raw && typeof raw === "object" ? raw : {};
  const kind = LAB_CONTEXT_KINDS.has(context.kind) ? context.kind : "component";
  return {
    kind,
    objectApiName: String(context.objectApiName || "Account"),
    recordId: String(context.recordId || model.sampleRecordId || ""),
    appName: String(context.appName || "Local"),
    stateKey: String(context.stateKey || ""),
    stateValue: String(context.stateValue || ""),
  };
}

function normalize(value) {
  return String(value || "").trim().toLowerCase();
}

function normalizeFormFactor(value) {
  const match = Array.from(FORM_FACTORS).find((option) => normalize(option) === normalize(value));
  return match || "Large";
}

function normalizeLabViewMode(value) {
  return normalize(value) === "setup" ? "setup" : "preview";
}

function normalizeLabFitMode(value) {
  const match = Array.from(FIT_MODES).find((option) => normalize(option) === normalize(value));
  return match || "fit";
}

function isContextProperty(name) {
  return CONTEXT_PROP_NAMES.has(name);
}
