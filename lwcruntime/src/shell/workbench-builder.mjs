import { emitPageReference } from "./navigation-service.mjs";

const TARGET_BY_KIND = {
  appPage: "lightning__AppPage",
  homePage: "lightning__HomePage",
  recordPage: "lightning__RecordPage",
  tab: "lightning__Tab",
  urlAddressable: "lightning__UrlAddressable",
  quickAction: "lightning__RecordAction",
  communityPage: "lightningCommunity__Page",
  utilityBar: "lightning__UtilityBar",
  flowScreen: "lightning__FlowScreen",
  flowAction: "lightning__FlowAction",
};

const PAGE_LABEL_BY_KIND = {
  appPage: "Draft App Page",
  homePage: "Draft Home Page",
  recordPage: "Draft Record Page",
  tab: "Draft Tab",
  urlAddressable: "Draft URL Page",
  quickAction: "Draft Action",
  communityPage: "Draft Community Page",
  utilityBar: "Draft Utility Bar",
  flowScreen: "Draft Flow Screen",
  flowAction: "Draft Flow Action",
};

const APP_LABEL_BY_KIND = {
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

const BUILDER_STORAGE_KEY = "glade:workbench-builder:v1";

let nextHostId = 0;

export function bootWorkbenchBuilder(root = document.body, config = {}) {
  const builder = root.querySelector("[data-glade-workbench-builder]");
  if (!builder) {
    return null;
  }
  const model = readWorkbenchModel();
  const state = {
    kind: "appPage",
    layout: "single",
    viewMode: "setup",
    components: [],
    objectSearchRequest: 0,
    recordSearchRequest: 0,
  };
  const controls = {
    kind: builder.querySelector("[data-glade-page-kind]"),
    targetPicker: builder.querySelector("[data-glade-target-picker]"),
    componentPicker: builder.querySelector("[data-glade-component-picker]"),
    object: builder.querySelector("[data-glade-object-input]"),
    objectResults: builder.querySelector("[data-glade-object-results]"),
    record: builder.querySelector("[data-glade-record-input]"),
    recordResults: builder.querySelector("[data-glade-record-results]"),
    sampleRecord: builder.querySelector("[data-glade-sample-record]"),
    app: builder.querySelector("[data-glade-app-input]"),
    appLabel: builder.querySelector("[data-glade-app-label]"),
    community: builder.querySelector("[data-glade-community-selector]"),
    formFactor: builder.querySelector("[data-glade-form-factor]"),
    formFactorOptions: Array.from(builder.querySelectorAll("[data-glade-form-factor-option]")),
    canvasFormFactor: builder.querySelector("[data-glade-canvas-form-factor]"),
    layout: builder.querySelector("[data-glade-layout-picker]"),
    canvas: builder.querySelector("[data-glade-page-layout]"),
    consoleMode: builder.querySelector("[data-glade-console-mode]"),
    stateKey: builder.querySelector("[data-glade-state-key]"),
    stateValue: builder.querySelector("[data-glade-state-value]"),
    flowInputs: builder.querySelector("[data-glade-flow-inputs]"),
    search: builder.querySelector("[data-glade-component-search]"),
    contextSummary: builder.querySelector("[data-glade-context-summary]"),
    contextGroups: Array.from(builder.querySelectorAll("[data-glade-context-group]")),
    title: builder.querySelector("[data-glade-draft-title]"),
    catalogCount: builder.querySelector("[data-glade-catalog-count]"),
    status: builder.querySelector("[data-glade-draft-status]"),
    clear: builder.querySelector("[data-glade-clear-draft]"),
    viewOptions: Array.from(builder.querySelectorAll("[data-glade-builder-view-option]")),
  };
  restoreBuilderDraft(state, controls);

  const render = () => renderDraft(builder, model, state, controls, config);
  builder.addEventListener("click", (event) => {
    const viewOption = event.target.closest("[data-glade-builder-view-option]");
    if (viewOption) {
      event.preventDefault();
      state.viewMode = normalizeBuilderViewMode(viewOption.dataset.gladeBuilderViewOption);
      applyBuilderViewState(builder, state, controls);
      persistBuilderDraft(state, controls);
      return;
    }
    const add = event.target.closest("[data-glade-add-component]");
    if (add) {
      event.preventDefault();
      if (add.disabled) {
        return;
      }
      state.components.push({
        qualifiedName: add.dataset.gladeAddComponent,
        region: add.dataset.gladeRegion || "main",
      });
      render();
      return;
    }
    const remove = event.target.closest("[data-glade-remove-component]");
    if (remove) {
      event.preventDefault();
      const index = Number(remove.dataset.gladeRemoveComponent);
      if (Number.isInteger(index)) {
        state.components.splice(index, 1);
        render();
      }
      return;
    }
    if (event.target.closest("[data-glade-clear-draft]")) {
      event.preventDefault();
      state.components = [];
      render();
      return;
    }
    if (event.target.closest("[data-glade-sample-record]")) {
      event.preventDefault();
      if (controls.record) {
        controls.record.value = model.sampleRecordId || "001000000000001AAA";
      }
      clearSearchResults(controls.recordResults, controls.record);
      render();
      return;
    }
    const objectResult = event.target.closest("[data-glade-object-result]");
    if (objectResult) {
      event.preventDefault();
      if (controls.object) {
        controls.object.value = objectResult.dataset.gladeApiName || "";
      }
      if (controls.record) {
        controls.record.value = "";
      }
      clearSearchResults(controls.objectResults, controls.object);
      clearSearchResults(controls.recordResults, controls.record);
      render();
      return;
    }
    const recordResult = event.target.closest("[data-glade-record-result]");
    if (recordResult) {
      event.preventDefault();
      if (controls.record) {
        controls.record.value = recordResult.dataset.gladeRecordId || "";
      }
      clearSearchResults(controls.recordResults, controls.record);
      render();
      return;
    }
    const formFactor = event.target.closest("[data-glade-form-factor-option]");
    if (formFactor) {
      event.preventDefault();
      if (controls.formFactor) {
        controls.formFactor.value = formFactor.dataset.gladeFormFactorOption || "Large";
      }
      render();
    }
    if (!event.target.closest("[data-glade-combobox-shell]")) {
      closeSearchResults(controls.objectResults, controls.object);
      closeSearchResults(controls.recordResults, controls.record);
    }
  });
  builder.addEventListener("keydown", (event) => {
    if (event.target.closest("[data-glade-object-result]")) {
      handleSearchOptionKeydown(event, controls.objectResults, controls.object);
      return;
    }
    if (event.target.closest("[data-glade-record-result]")) {
      handleSearchOptionKeydown(event, controls.recordResults, controls.record);
    }
  });
  builder.addEventListener("dragstart", (event) => {
    const card = event.target.closest("[data-glade-drag-component]");
    if (!card || !event.dataTransfer) {
      return;
    }
    event.dataTransfer.effectAllowed = "copy";
    event.dataTransfer.setData("text/plain", card.dataset.gladeDragComponent || "");
    event.dataTransfer.setData("application/x-glade-lwc", card.dataset.gladeDragComponent || "");
  });
  builder.addEventListener("dragover", (event) => {
    const region = event.target.closest("[data-glade-region-drop]");
    if (!region || region.hidden) {
      return;
    }
    event.preventDefault();
    if (event.dataTransfer) {
      event.dataTransfer.dropEffect = "copy";
    }
  });
  builder.addEventListener("drop", (event) => {
    const region = event.target.closest("[data-glade-region-drop]");
    if (!region || region.hidden || !event.dataTransfer) {
      return;
    }
    const qualifiedName = event.dataTransfer.getData("application/x-glade-lwc") || event.dataTransfer.getData("text/plain");
    if (!qualifiedName) {
      return;
    }
    event.preventDefault();
    state.components.push({
      qualifiedName,
      region: region.dataset.gladeRegionDrop || "main",
    });
    render();
  });
  for (const control of [
    controls.kind,
    controls.componentPicker,
    controls.object,
    controls.record,
    controls.app,
    controls.community,
    controls.formFactor,
    controls.layout,
    controls.consoleMode,
    controls.stateKey,
    controls.stateValue,
    controls.flowInputs,
    controls.search,
  ]) {
    if (control) {
      control.addEventListener("input", render);
      control.addEventListener("change", render);
    }
  }
  if (controls.object) {
    controls.object.addEventListener("input", () => handleObjectInput(state, controls));
    controls.object.addEventListener("focus", () => refreshObjectSearch(state, controls));
    controls.object.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        closeSearchResults(controls.objectResults, controls.object);
        return;
      }
      if (event.key === "ArrowDown" && focusFirstSearchResult(controls.objectResults)) {
        event.preventDefault();
      }
    });
  }
  if (controls.record) {
    controls.record.addEventListener("input", () => refreshRecordSearch(state, controls));
    controls.record.addEventListener("focus", () => refreshRecordSearch(state, controls));
    controls.record.addEventListener("keydown", (event) => {
      if (event.key === "Escape") {
        closeSearchResults(controls.recordResults, controls.record);
        return;
      }
      if (event.key === "ArrowDown" && focusFirstSearchResult(controls.recordResults)) {
        event.preventDefault();
      }
    });
  }
  render();
  return { model, state, render };
}

function handleObjectInput(state, controls) {
  if (!controls.object?.value?.trim()) {
    if (controls.record) {
      controls.record.value = "";
    }
    state.recordSearchRequest += 1;
    clearSearchResults(controls.recordResults, controls.record);
  }
  refreshObjectSearch(state, controls);
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

function renderDraft(builder, model, state, controls, config) {
  state.kind = controls.kind?.value || state.kind || "appPage";
  state.layout = controls.layout?.value || state.layout || "mainSidebar";
  state.viewMode = normalizeBuilderViewMode(state.viewMode);
  const formFactor = currentFormFactor(controls);
  const target = TARGET_BY_KIND[state.kind] || TARGET_BY_KIND.appPage;
  if (controls.componentPicker?.value && controls.search && controls.search.value !== controls.componentPicker.value) {
    controls.search.value = controls.componentPicker.value;
  }
  updateContextControls(state, controls);
  updateLayout(builder, state, controls);
  applyBuilderViewState(builder, state, controls);
  updateCatalog(builder, model, target, controls.search?.value, formFactor);
  state.components = state.components.filter((placement) => componentSupportsTarget(findComponent(model, placement.qualifiedName), target));
  if (!sidebarAvailable(state.layout, formFactor)) {
    state.components = state.components.map((placement) => placement.region === "sidebar" ? { ...placement, region: "main" } : placement);
  }
  updateContextScripts(state, controls, config);
  for (const region of builder.querySelectorAll("[data-glade-region-items]")) {
    region.replaceChildren();
  }
  const pageReference = currentDraftPageReference(state, controls);
  emitPageReference(pageReference);
  const title = controls.title;
  if (title) {
    title.textContent = PAGE_LABEL_BY_KIND[state.kind] || PAGE_LABEL_BY_KIND.appPage;
  }
  const enabledCount = model.components?.filter((component) => componentSupportsTarget(component, target)).length || 0;
  if (controls.catalogCount) {
    controls.catalogCount.textContent = String(enabledCount);
  }
  if (controls.status) {
    controls.status.textContent = `${state.components.length} placed / ${enabledCount} available`;
  }
  updateFormFactorButtons(controls);
  document.body.dataset.gladeBuilderConsole = String(Boolean(controls.consoleMode?.checked));
  state.components.forEach((placement, index) => renderPlacement(builder, model, placement, index, state, controls, target));
  persistBuilderDraft(state, controls);
}

function restoreBuilderDraft(state, controls) {
  const draft = readBuilderDraft();
  if (!draft) {
    return;
  }
  setControlValue(controls.kind, draft.kind);
  setControlValue(controls.componentPicker, draft.componentPicker);
  setControlValue(controls.object, draft.objectApiName);
  setControlValue(controls.record, draft.recordId);
  setControlValue(controls.app, draft.appName);
  setControlValue(controls.community, draft.communitySite);
  setControlValue(controls.formFactor, draft.formFactor);
  setControlValue(controls.layout, draft.layout);
  setControlValue(controls.stateKey, draft.stateKey);
  setControlValue(controls.stateValue, draft.stateValue);
  setControlValue(controls.flowInputs, draft.flowInputs);
  setControlValue(controls.search, draft.search);
  if (controls.consoleMode && typeof draft.consoleMode === "boolean") {
    controls.consoleMode.checked = draft.consoleMode;
  }
  state.kind = controls.kind?.value || draft.kind || state.kind;
  state.layout = controls.layout?.value || draft.layout || state.layout;
  state.viewMode = normalizeBuilderViewMode(draft.viewMode);
  state.components = Array.isArray(draft.components)
    ? draft.components
        .filter((placement) => placement && typeof placement.qualifiedName === "string")
        .map((placement) => ({
          qualifiedName: placement.qualifiedName,
          region: placement.region || "main",
        }))
    : [];
}

function readBuilderDraft() {
  if (typeof sessionStorage === "undefined") {
    return null;
  }
  try {
    const draft = JSON.parse(sessionStorage.getItem(BUILDER_STORAGE_KEY) || "null");
    return draft && typeof draft === "object" ? draft : null;
  } catch (_err) {
    return null;
  }
}

function persistBuilderDraft(state, controls) {
  if (typeof sessionStorage === "undefined") {
    return;
  }
  const draft = {
    kind: state.kind,
    layout: state.layout,
    viewMode: normalizeBuilderViewMode(state.viewMode),
    componentPicker: controls.componentPicker?.value || "",
    objectApiName: controls.object?.value || "",
    recordId: controls.record?.value || "",
    appName: controls.app?.value || "",
    communitySite: controls.community?.value || "",
    formFactor: currentFormFactor(controls),
    stateKey: controls.stateKey?.value || "",
    stateValue: controls.stateValue?.value || "",
    flowInputs: controls.flowInputs?.value || "",
    search: controls.search?.value || "",
    consoleMode: Boolean(controls.consoleMode?.checked),
    components: state.components.map((placement) => ({
      qualifiedName: placement.qualifiedName,
      region: placement.region || "main",
    })),
  };
  try {
    sessionStorage.setItem(BUILDER_STORAGE_KEY, JSON.stringify(draft));
  } catch (_err) {
    // Draft persistence should never affect the builder.
  }
}

function normalizeBuilderViewMode(value) {
  return value === "preview" ? "preview" : "setup";
}

function applyBuilderViewState(builder, state, controls) {
  state.viewMode = normalizeBuilderViewMode(state.viewMode);
  builder.dataset.gladeBuilderView = state.viewMode;
  for (const option of controls.viewOptions || []) {
    option.setAttribute("aria-pressed", String(option.dataset.gladeBuilderViewOption === state.viewMode));
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

function updateCatalog(builder, model, target, query, formFactor = "Large") {
  const search = normalize(query);
  const layout = builder.dataset.gladeLayout || "mainSidebar";
  for (const card of builder.querySelectorAll("[data-glade-component-card]")) {
    const component = findComponent(model, card.dataset.gladeComponent);
    const supported = componentSupportsTarget(component, target);
    const matched = componentMatchesSearch(component, search);
    card.dataset.gladeComponentSupported = String(supported);
    card.dataset.gladeComponentMatches = String(matched);
    card.hidden = !matched;
    for (const button of card.querySelectorAll("[data-glade-add-component]")) {
      const regionAvailable = button.dataset.gladeRegion !== "sidebar" || sidebarAvailable(layout, formFactor);
      button.disabled = !supported || !regionAvailable;
      button.setAttribute("aria-disabled", String(button.disabled));
    }
  }
}

function updateLayout(builder, state, controls) {
  builder.dataset.gladeLayout = state.layout;
  const formFactor = currentFormFactor(controls);
  if (controls.canvas) {
    controls.canvas.dataset.gladeLayout = state.layout;
    controls.canvas.dataset.gladeFormFactor = formFactor;
  }
  if (controls.canvasFormFactor) {
    controls.canvasFormFactor.textContent = formFactor;
  }
  for (const region of builder.querySelectorAll("[data-glade-region-drop]")) {
    region.hidden = region.dataset.gladeRegionDrop === "sidebar" && !sidebarAvailable(state.layout, formFactor);
  }
}

function currentFormFactor(controls) {
  return controls.formFactor?.value || "Large";
}

function sidebarAvailable(layout, formFactor) {
  return layout !== "single" && formFactor !== "Small";
}

function updateContextControls(state, controls) {
  const recordEnabled = targetUsesRecordContext(state.kind);
  const flowEnabled = targetUsesFlowContext(state.kind);
  for (const group of controls.contextGroups || []) {
    const name = group.dataset.gladeContextGroup;
    const visible = name === "record" ? recordEnabled : name === "flow" ? flowEnabled : true;
    group.hidden = !visible;
    for (const input of group.querySelectorAll("input, select, textarea, button")) {
      if (input.matches("[data-glade-console-mode]")) {
        continue;
      }
      input.disabled = !visible;
    }
  }
  if (controls.appLabel) {
    controls.appLabel.textContent = APP_LABEL_BY_KIND[state.kind] || "App / page";
  }
  if (controls.contextSummary) {
    controls.contextSummary.textContent = contextSummaryText(state, controls);
  }
  if (!recordEnabled) {
    clearSearchResults(controls.objectResults, controls.object);
    clearSearchResults(controls.recordResults, controls.record);
  }
}

function targetUsesRecordContext(kind) {
  return kind === "recordPage" || kind === "quickAction";
}

function targetUsesFlowContext(kind) {
  return kind === "flowScreen" || kind === "flowAction";
}

function targetUsesAppContext(kind) {
  return kind !== "";
}

function contextSummaryText(state, controls) {
  if (targetUsesRecordContext(state.kind)) {
    const objectName = controls.object?.value || "Object";
    const recordId = controls.record?.value || "no record selected";
    return `${objectName} / ${recordId}`;
  }
  if (targetUsesFlowContext(state.kind)) {
    return controls.app?.value ? `Flow ${controls.app.value}` : "Flow context";
  }
  if (state.kind === "homePage") {
    return "Home page context without record data";
  }
  if (state.kind === "appPage") {
    return controls.app?.value ? `App ${controls.app.value}` : "App page context";
  }
  if (state.kind === "tab") {
    return controls.app?.value ? `Tab ${controls.app.value}` : "Tab context";
  }
  return PAGE_LABEL_BY_KIND[state.kind] || "Page context";
}

function updateContextScripts(state, controls, config) {
  const context = currentDraftContext(state, controls);
  const pageReference = currentDraftPageReference(state, controls);
  const contextNode = document.getElementById("glade-lwc-context");
  if (contextNode) {
    contextNode.textContent = JSON.stringify(context);
  }
  const configNode = document.getElementById("glade-lightning-config");
  if (configNode) {
    configNode.textContent = JSON.stringify({ ...config, pageReference });
  }
  document.dispatchEvent(new CustomEvent("glade:context-changed", { detail: { context, pageReference } }));
}

function renderPlacement(builder, model, placement, index, state, controls, target) {
  const component = findComponent(model, placement.qualifiedName);
  if (!component || !componentSupportsTarget(component, target)) {
    return;
  }
  const region = builder.querySelector(`[data-glade-region-items="${cssEscape(placement.region)}"]`);
  if (!region) {
    return;
  }
  const hostId = `glade-draft-lwc-${nextHostId++}`;
  const frame = document.createElement("article");
  frame.className = "glade-draft-component";
  frame.dataset.gladeDraftComponent = component.qualifiedName;
  frame.innerHTML = `<header><strong></strong><code></code><button class="glade-shell-button" type="button">Remove</button></header><div class="glade-host"></div>`;
  frame.querySelector("strong").textContent = component.label || component.name || component.qualifiedName;
  frame.querySelector("code").textContent = component.qualifiedName;
  const remove = frame.querySelector("button");
  remove.dataset.gladeRemoveComponent = String(index);
  const host = frame.querySelector(".glade-host");
  host.id = hostId;
  region.append(frame);
  const attrs = {
    ...defaultTargetProperties(component, target),
    ...currentDraftAttrs(state, controls),
  };
  window.$Lightning.createComponent(component.qualifiedName, attrs, hostId, (_cmp, status, message) => {
    if (status === "SUCCESS") {
      return;
    }
    renderMountError(host, `Unable to mount ${component.qualifiedName}`, message || "The local runtime did not return a successful mount status.");
  });
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

function currentDraftContext(state, controls) {
  const statePairs = {};
  const stateKey = controls.stateKey?.value?.trim();
  if (stateKey) {
    statePairs[stateKey] = controls.stateValue?.value || "";
  }
  const recordContext = targetUsesRecordContext(state.kind);
  const flowContext = targetUsesFlowContext(state.kind);
  return {
    kind: state.kind,
    recordId: recordContext ? controls.record?.value || "" : "",
    objectApiName: recordContext ? controls.object?.value || "" : "",
    appName: targetUsesAppContext(state.kind) ? controls.app?.value || "" : "",
    formFactor: currentFormFactor(controls),
    state: statePairs,
    community: {
      site: controls.community?.value || "",
    },
    flow: {
      apiName: flowContext ? controls.app?.value || "" : "",
    },
  };
}

function currentDraftAttrs(state, controls) {
  const ctx = currentDraftContext(state, controls);
  const attrs = {
    formFactor: ctx.formFactor,
  };
  if (ctx.recordId) {
    attrs.recordId = ctx.recordId;
  }
  if (ctx.objectApiName) {
    attrs.objectApiName = ctx.objectApiName;
  }
  if (ctx.appName) {
    attrs.appName = ctx.appName;
  }
  if (ctx.community?.site) {
    attrs.communitySite = ctx.community.site;
  }
  if (controls.flowInputs?.value) {
    attrs.flowInputs = controls.flowInputs.value;
  }
  return attrs;
}

function currentDraftPageReference(state, controls) {
  const baseState = {};
  const stateKey = controls.stateKey?.value?.trim();
  if (stateKey) {
    baseState[stateKey] = controls.stateValue?.value || "";
  }
  switch (state.kind) {
    case "recordPage":
      return {
        type: "standard__recordPage",
        attributes: {
          objectApiName: controls.object?.value || "",
          recordId: controls.record?.value || "",
          actionName: "view",
        },
        state: baseState,
      };
    case "tab":
      return {
        type: "standard__navItemPage",
        attributes: { apiName: controls.app?.value || "Local" },
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
        attributes: { componentName: controls.componentPicker?.value || "" },
        state: baseState,
      };
    case "quickAction":
      return {
        type: "standard__quickAction",
        attributes: {
          apiName: controls.app?.value || "",
          objectApiName: controls.object?.value || "",
          recordId: controls.record?.value || "",
        },
        state: baseState,
      };
    case "communityPage":
      return {
        type: "comm__namedPage",
        attributes: { name: controls.app?.value || "Home" },
        state: baseState,
      };
    case "utilityBar":
      return {
        type: "standard__component",
        attributes: { componentName: controls.componentPicker?.value || "" },
        state: baseState,
      };
    case "flowScreen":
      return {
        type: "standard__flow",
        attributes: {
          flowApiName: controls.app?.value || "",
        },
        state: baseState,
      };
    case "flowAction":
      return {
        type: "standard__flow",
        attributes: {
          flowApiName: controls.app?.value || "",
        },
        state: baseState,
      };
    default:
      return {
        type: "standard__app",
        attributes: { appTarget: controls.app?.value || "Local" },
        state: baseState,
      };
  }
}

async function refreshObjectSearch(state, controls) {
  if (!controls.objectResults || !targetUsesRecordContext(state.kind)) {
    clearSearchResults(controls.objectResults, controls.object);
    return;
  }
  const query = controls.object?.value || "";
  const request = ++state.objectSearchRequest;
  try {
    const response = await fetch(`/lightning/local/objects.json?q=${encodeURIComponent(query)}`, {
      headers: { Accept: "application/json" },
    });
    if (!response.ok || request !== state.objectSearchRequest) {
      return;
    }
    const payload = await response.json();
    if (request !== state.objectSearchRequest) {
      return;
    }
    renderObjectResults(controls, payload.objects || []);
  } catch (_err) {
    clearSearchResults(controls.objectResults, controls.object);
  }
}

async function refreshRecordSearch(state, controls) {
  if (!controls.recordResults || !targetUsesRecordContext(state.kind)) {
    clearSearchResults(controls.recordResults, controls.record);
    return;
  }
  const objectName = controls.object?.value?.trim();
  if (!objectName) {
    clearSearchResults(controls.recordResults, controls.record);
    return;
  }
  const query = controls.record?.value || "";
  const request = ++state.recordSearchRequest;
  try {
    const params = new URLSearchParams({ object: objectName, q: query });
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
    renderRecordResults(controls, payload.records || []);
  } catch (_err) {
    clearSearchResults(controls.recordResults, controls.record);
  }
}

function renderObjectResults(controls, objects) {
  clearSearchResults(controls.objectResults, controls.object);
  if (!controls.objectResults || objects.length === 0) {
    return;
  }
  openSearchResults(controls.objectResults, controls.object);
  for (const [index, object] of objects.slice(0, 25).entries()) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "glade-search-result";
    button.dataset.gladeObjectResult = "";
    button.dataset.gladeApiName = object.apiName || "";
    button.id = `glade-object-result-${index}`;
    button.setAttribute("role", "option");
    button.setAttribute("aria-selected", "false");
    const label = object.label || object.apiName || "Object";
    button.innerHTML = `<strong></strong><span></span>`;
    button.querySelector("strong").textContent = label;
    button.querySelector("span").textContent = `${object.apiName || ""}${object.recordCount ? ` · ${object.recordCount} records` : ""}`;
    controls.objectResults.append(button);
  }
}

function renderRecordResults(controls, records) {
  clearSearchResults(controls.recordResults, controls.record);
  if (!controls.recordResults || records.length === 0) {
    return;
  }
  openSearchResults(controls.recordResults, controls.record);
  for (const [index, record] of records.slice(0, 25).entries()) {
    const button = document.createElement("button");
    button.type = "button";
    button.className = "glade-search-result";
    button.dataset.gladeRecordResult = "";
    button.dataset.gladeRecordId = record.id || "";
    button.id = `glade-record-result-${index}`;
    button.setAttribute("role", "option");
    button.setAttribute("aria-selected", "false");
    button.innerHTML = `<strong></strong><span></span>`;
    button.querySelector("strong").textContent = record.title || record.id || "Record";
    button.querySelector("span").textContent = record.id || "";
    controls.recordResults.append(button);
  }
}

function openSearchResults(node, input) {
  if (node) {
    node.hidden = false;
  }
  if (input) {
    input.setAttribute("aria-expanded", "true");
  }
}

function closeSearchResults(node, input) {
  if (node) {
    node.hidden = true;
  }
  if (input) {
    input.setAttribute("aria-expanded", "false");
  }
}

function clearSearchResults(node, input) {
  if (node) {
    node.replaceChildren();
  }
  closeSearchResults(node, input);
}

function focusFirstSearchResult(node) {
  if (!node || node.hidden) {
    return false;
  }
  const first = node.querySelector(".glade-search-result");
  if (!first) {
    return false;
  }
  first.focus();
  return true;
}

function handleSearchOptionKeydown(event, results, input) {
  const current = event.target.closest(".glade-search-result");
  if (!current || !results?.contains(current)) {
    return;
  }
  if (event.key === "Enter" || event.key === " ") {
    event.preventDefault();
    current.click();
    return;
  }
  if (event.key === "Escape") {
    event.preventDefault();
    closeSearchResults(results, input);
    input?.focus();
    return;
  }
  if (event.key !== "ArrowDown" && event.key !== "ArrowUp") {
    return;
  }
  event.preventDefault();
  const options = Array.from(results.querySelectorAll(".glade-search-result"));
  const index = options.indexOf(current);
  if (index === -1 || options.length === 0) {
    return;
  }
  const nextIndex = event.key === "ArrowDown"
    ? (index + 1) % options.length
    : (index - 1 + options.length) % options.length;
  options[nextIndex]?.focus();
}

function findComponent(model, qualifiedName) {
  const want = normalize(qualifiedName);
  return (model.components || []).find((component) => normalize(component.qualifiedName) === want);
}

function componentSupportsTarget(component, target) {
  if (!component || !component.exposed) {
    return false;
  }
  return (component.targets || []).some((candidate) => normalize(candidate) === normalize(target));
}

function componentMatchesSearch(component, search) {
  if (!search) {
    return true;
  }
  const haystack = [
    component?.label,
    component?.name,
    component?.qualifiedName,
    ...(component?.targets || []),
  ].join(" ");
  return normalize(haystack).includes(search);
}

function defaultTargetProperties(component, target) {
  const support = (component.targetSupport || []).find((candidate) => normalize(candidate.target) === normalize(target));
  return { ...(support?.properties || {}) };
}

function updateFormFactorButtons(controls) {
  const selected = controls.formFactor?.value || "Large";
  for (const button of controls.formFactorOptions || []) {
    const active = button.dataset.gladeFormFactorOption === selected;
    button.dataset.gladeSelected = String(active);
    button.setAttribute("aria-pressed", String(active));
  }
}

function normalize(value) {
  return String(value || "").trim().toLowerCase();
}

function cssEscape(value) {
  if (window.CSS && typeof window.CSS.escape === "function") {
    return window.CSS.escape(value);
  }
  return String(value || "").replace(/"/g, '\\"');
}
