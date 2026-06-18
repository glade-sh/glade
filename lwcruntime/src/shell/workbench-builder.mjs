import { emitPageReference } from "./navigation-service.mjs";

const TARGET_BY_KIND = {
  appPage: "lightning__AppPage",
  homePage: "lightning__HomePage",
  recordPage: "lightning__RecordPage",
  tab: "lightning__Tab",
};

const PAGE_LABEL_BY_KIND = {
  appPage: "Draft App Page",
  homePage: "Draft Home Page",
  recordPage: "Draft Record Page",
  tab: "Draft Tab",
};

let nextHostId = 0;

export function bootWorkbenchBuilder(root = document.body, config = {}) {
  const builder = root.querySelector("[data-glade-workbench-builder]");
  if (!builder) {
    return null;
  }
  const model = readWorkbenchModel();
  const state = {
    kind: "appPage",
    components: [],
  };
  const controls = {
    kind: builder.querySelector("[data-glade-page-kind]"),
    object: builder.querySelector("[data-glade-object-input]"),
    record: builder.querySelector("[data-glade-record-input]"),
    app: builder.querySelector("[data-glade-app-input]"),
    formFactor: builder.querySelector("[data-glade-form-factor]"),
    search: builder.querySelector("[data-glade-component-search]"),
    title: builder.querySelector("[data-glade-draft-title]"),
    status: builder.querySelector("[data-glade-draft-status]"),
    clear: builder.querySelector("[data-glade-clear-draft]"),
  };

  const render = () => renderDraft(builder, model, state, controls, config);
  builder.addEventListener("click", (event) => {
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
    }
  });
  for (const control of [controls.kind, controls.object, controls.record, controls.app, controls.formFactor, controls.search]) {
    if (control) {
      control.addEventListener("input", render);
      control.addEventListener("change", render);
    }
  }
  render();
  return { model, state, render };
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
  const target = TARGET_BY_KIND[state.kind] || TARGET_BY_KIND.appPage;
  updateCatalog(builder, model, target, controls.search?.value);
  state.components = state.components.filter((placement) => componentSupportsTarget(findComponent(model, placement.qualifiedName), target));
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
  if (controls.status) {
    controls.status.textContent = `${state.components.length} placed / ${enabledCount} available`;
  }
  state.components.forEach((placement, index) => renderPlacement(builder, model, placement, index, state, controls, target));
}

function updateCatalog(builder, model, target, query) {
  const search = normalize(query);
  for (const card of builder.querySelectorAll("[data-glade-component-card]")) {
    const component = findComponent(model, card.dataset.gladeComponent);
    const supported = componentSupportsTarget(component, target);
    const matched = componentMatchesSearch(component, search);
    card.dataset.gladeComponentSupported = String(supported);
    card.dataset.gladeComponentMatches = String(matched);
    card.hidden = !matched;
    for (const button of card.querySelectorAll("[data-glade-add-component]")) {
      button.disabled = !supported;
      button.setAttribute("aria-disabled", String(!supported));
    }
  }
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
  frame.innerHTML = `<header><strong></strong><code></code><button type="button">Remove</button></header><div class="glade-host"></div>`;
  frame.querySelector("strong").textContent = component.label || component.name || component.qualifiedName;
  frame.querySelector("code").textContent = component.qualifiedName;
  const remove = frame.querySelector("button");
  remove.dataset.gladeRemoveComponent = String(index);
  const host = frame.querySelector(".glade-host");
  host.id = hostId;
  region.append(frame);
  const attrs = {
    ...currentDraftAttrs(state, controls),
    ...defaultTargetProperties(component, target),
  };
  window.$Lightning.createComponent(component.qualifiedName, attrs, hostId, (_cmp, status, message) => {
    if (status === "SUCCESS") {
      return;
    }
    host.textContent = message || `Unable to mount ${component.qualifiedName}`;
  });
}

function currentDraftContext(state, controls) {
  return {
    kind: state.kind,
    recordId: controls.record?.value || "",
    objectApiName: controls.object?.value || "",
    appName: controls.app?.value || "",
    formFactor: controls.formFactor?.value || "Large",
    community: {},
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
  return attrs;
}

function currentDraftPageReference(state, controls) {
  const baseState = {};
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
    default:
      return {
        type: "standard__app",
        attributes: { appTarget: controls.app?.value || "Local" },
        state: baseState,
      };
  }
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

function normalize(value) {
  return String(value || "").trim().toLowerCase();
}

function cssEscape(value) {
  if (window.CSS && typeof window.CSS.escape === "function") {
    return window.CSS.escape(value);
  }
  return String(value || "").replace(/"/g, '\\"');
}
