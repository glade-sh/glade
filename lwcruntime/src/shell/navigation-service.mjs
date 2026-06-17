import { reportDiagnostic } from "./diagnostics.mjs";

const DEFAULT_PAGE_REFERENCE = {
  type: "standard__namedPage",
  attributes: { pageName: "home" },
  state: {},
};

const SUPPORTED_PAGE_REFERENCE_TYPES = new Set([
  "standard__recordPage",
  "standard__objectPage",
  "standard__recordRelationshipPage",
  "standard__navItemPage",
  "standard__app",
  "standard__namedPage",
  "standard__component",
  "standard__quickAction",
  "standard__webPage",
  "comm__namedPage",
  "comm__loginPage",
  "comm__managedContentPage",
  "comm__recordPage",
  "comm__recordRelationshipPage",
]);

const pageReferenceListeners = new Set();

export { SUPPORTED_PAGE_REFERENCE_TYPES };

export class CurrentPageReferenceAdapter {
  constructor(dataCallback) {
    this.dataCallback = dataCallback;
    this.unsubscribe = null;
  }

  connect() {
    if (typeof this.dataCallback === "function") {
      this.dataCallback(currentPageReference());
    }
    this.unsubscribe = subscribePageReference((pageReference) => {
      if (typeof this.dataCallback === "function") {
        this.dataCallback(pageReference);
      }
    });
  }

  update() {
    if (typeof this.dataCallback === "function") {
      this.dataCallback(currentPageReference());
    }
  }

  disconnect() {
    if (this.unsubscribe) {
      this.unsubscribe();
      this.unsubscribe = null;
    }
  }
}

export function readConfig() {
  const node = document.getElementById("glade-lightning-config");
  if (!node) {
    return {};
  }
  try {
    return JSON.parse(node.textContent || "{}");
  } catch (_err) {
    return {};
  }
}

export function currentPageReference() {
  return readConfig().pageReference || DEFAULT_PAGE_REFERENCE;
}

export function readShellContext() {
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

export function subscribePageReference(listener) {
  pageReferenceListeners.add(listener);
  return () => pageReferenceListeners.delete(listener);
}

export function emitPageReference(pageReference = currentPageReference()) {
  for (const listener of [...pageReferenceListeners]) {
    listener(pageReference);
  }
  document.dispatchEvent(new CustomEvent("glade:page-reference", { detail: pageReference }));
}

export async function generateUrl(pageReference) {
  return buildLocalUrl(pageReference);
}

export async function navigate(pageReference, options = {}) {
  const url = await generateUrl(pageReference);
  navigateToUrl(url, options);
  return url;
}

export function navigateToUrl(url, options = {}) {
  if (!url || url === currentPath()) {
    return;
  }
  const assign = options.assign || ((nextUrl) => window.location.assign(nextUrl));
  assign(url);
}

export function buildLocalUrl(pageReference) {
  const ref = pageReference || {};
  const attrs = ref.attributes || {};
  const state = ref.state || {};
  if (!SUPPORTED_PAGE_REFERENCE_TYPES.has(ref.type)) {
    const code = String(ref.type || "").startsWith("comm__") ? "GLADELWC103" : "GLADELWC040";
    throw navigationError(code, `${code} navigation target unsupported`, ref);
  }
  switch (ref.type) {
    case "standard__recordPage":
      return recordPageUrl(attrs, state);
    case "standard__objectPage":
      return objectPageUrl(attrs, state, ref);
    case "standard__recordRelationshipPage":
      return recordRelationshipUrl(attrs, state);
    case "standard__navItemPage":
      return requiredUrl(attrs.apiName, "GLADELWC040", ref, () => `/lwc/preview/tab/${enc(attrs.apiName)}`);
    case "standard__app":
      return appUrl(attrs, state, ref);
    case "standard__namedPage":
      return namedPageUrl(attrs, state, ref);
    case "standard__component":
      return requiredUrl(attrs.componentName, "GLADELWC040", ref, () => withState(`/lwc/preview/cmp/${componentPath(attrs.componentName)}`, state));
    case "standard__quickAction":
      return quickActionUrl(attrs, state, ref);
    case "standard__webPage":
      return requiredUrl(attrs.url, "GLADELWC040", ref, () => String(attrs.url));
    case "comm__namedPage":
      return communityNamedPageUrl(attrs, state, ref);
    case "comm__loginPage":
      return communityLoginPageUrl(attrs, state, ref);
    case "comm__managedContentPage":
      return communityManagedContentPageUrl(attrs, state, ref);
    case "comm__recordPage":
      return communityRecordPageUrl(attrs, state, ref);
    case "comm__recordRelationshipPage":
      return communityRecordRelationshipPageUrl(attrs, state, ref);
    default:
      throw navigationError("GLADELWC040", "GLADELWC040 navigation target unsupported", ref);
  }
}

function recordPageUrl(attrs, state) {
  if (!attrs.recordId) {
    throw navigationError("GLADELWC040", "GLADELWC040 navigation target unsupported: recordId is required", { attributes: attrs });
  }
  const objectApiName = attrs.objectApiName || "Record";
  return withState(`/lwc/preview/record/${enc(objectApiName)}/${enc(attrs.recordId)}`, state);
}

function recordRelationshipUrl(attrs, state) {
  if (!attrs.recordId || !attrs.relationshipApiName) {
    throw navigationError("GLADELWC040", "GLADELWC040 navigation target unsupported: recordId and relationshipApiName are required", { attributes: attrs });
  }
  const objectApiName = attrs.objectApiName || "Record";
  return withState(`/lwc/preview/record/${enc(objectApiName)}/${enc(attrs.recordId)}`, {
    ...state,
    relationship: attrs.relationshipApiName,
  });
}

function objectPageUrl(attrs, state, ref) {
  const actionName = attrs.actionName || "home";
  if (actionName !== "home" && actionName !== "list") {
    throw navigationError("GLADELWC042", "GLADELWC042 object page unsupported locally", ref);
  }
  if (!attrs.objectApiName) {
    throw navigationError("GLADELWC042", "GLADELWC042 object page unsupported locally: objectApiName is required", ref);
  }
  return withState(`/lwc/preview/app/${enc(attrs.objectApiName)}`, {
    ...state,
    object: attrs.objectApiName,
    actionName,
  });
}

function appUrl(attrs, state, ref) {
  const appTarget = attrs.appTarget || attrs.appDeveloperName || attrs.appName;
  if (!appTarget) {
    throw navigationError("GLADELWC040", "GLADELWC040 navigation target unsupported: appTarget is required", ref);
  }
  if (attrs.pageRef) {
    return withState(buildLocalUrl(attrs.pageRef), { ...state, app: appTarget });
  }
  return withState(`/lwc/preview/app/${enc(appTarget)}`, state);
}

function namedPageUrl(attrs, state, ref) {
  if (!attrs.pageName) {
    throw navigationError("GLADELWC040", "GLADELWC040 navigation target unsupported: pageName is required", ref);
  }
  if (attrs.pageName === "home") {
    return withState("/lwc/preview/home", state);
  }
  return withState(`/lwc/preview/app/${enc(attrs.pageName)}`, state);
}

function quickActionUrl(attrs, state, ref) {
  if (!attrs.apiName) {
    throw navigationError("GLADELWC041", "GLADELWC041 quick action context missing: apiName is required", ref);
  }
  const objectApiName = attrs.objectApiName || objectFromQuickAction(attrs.apiName);
  const actionName = actionFromQuickAction(attrs.apiName);
  if (!objectApiName && !attrs.recordId) {
    return withState(`/lwc/preview/action/global/${enc(actionName || attrs.apiName)}`, state);
  }
  if (!objectApiName || !attrs.recordId || !actionName) {
    throw navigationError("GLADELWC041", "GLADELWC041 quick action context missing", ref);
  }
  return withState(`/lwc/preview/action/${enc(objectApiName)}/${enc(attrs.recordId)}/${enc(actionName)}`, state);
}

function communityNamedPageUrl(attrs, state, ref) {
  const pageName = attrs.name || attrs.pageName;
  return requiredCommunityUrl(pageName, ref, (site) => withState(`/lwc/preview/community/${enc(site)}/${enc(pageName)}`, state));
}

function communityLoginPageUrl(attrs, state, ref) {
  const actionName = attrs.actionName || "login";
  return requiredCommunityUrl(actionName, ref, (site) => withState(`/lwc/preview/community/${enc(site)}/${enc(actionName)}`, state));
}

function communityManagedContentPageUrl(attrs, state, ref) {
  const contentKey = attrs.contentKey || attrs.contentId || attrs.managedContentId;
  const pageName = communityPageName(attrs, "managed-content");
  return requiredCommunityUrl(contentKey, ref, (site) => withState(`/lwc/preview/community/${enc(site)}/${enc(pageName)}`, {
    ...state,
    contentKey,
    contentType: "managedContent",
  }));
}

function communityRecordPageUrl(attrs, state, ref) {
  if (!attrs.recordId) {
    throw navigationError("GLADELWC103", "GLADELWC103 community navigation target unsupported: recordId is required", ref);
  }
  const objectApiName = attrs.objectApiName || "Record";
  const pageName = communityPageName(attrs, objectApiName);
  return requiredCommunityUrl(attrs.recordId, ref, (site) => withState(`/lwc/preview/community/${enc(site)}/${enc(pageName)}`, {
    ...state,
    recordId: attrs.recordId,
    objectApiName,
    actionName: attrs.actionName || "view",
  }));
}

function communityRecordRelationshipPageUrl(attrs, state, ref) {
  if (!attrs.recordId || !attrs.relationshipApiName) {
    throw navigationError("GLADELWC103", "GLADELWC103 community navigation target unsupported: recordId and relationshipApiName are required", ref);
  }
  const objectApiName = attrs.objectApiName || "Record";
  const pageName = communityPageName(attrs, objectApiName);
  return requiredCommunityUrl(attrs.recordId, ref, (site) => withState(`/lwc/preview/community/${enc(site)}/${enc(pageName)}`, {
    ...state,
    recordId: attrs.recordId,
    objectApiName,
    relationship: attrs.relationshipApiName,
  }));
}

function requiredCommunityUrl(requiredValue, ref, builder) {
  if (!requiredValue) {
    throw navigationError("GLADELWC103", "GLADELWC103 community navigation target unsupported", ref);
  }
  const site = communitySite(ref);
  if (!site) {
    throw navigationError("GLADELWC103", "GLADELWC103 community navigation target unsupported: site is required", ref);
  }
  return builder(site);
}

function communitySite(ref) {
  const attrs = ref?.attributes || {};
  return attrs.site || attrs.siteName || readShellContext().community?.site || "";
}

function communityPageName(attrs, fallback) {
  return attrs.pageName || attrs.name || readShellContext().pageName || fallback;
}

function requiredUrl(value, code, ref, builder) {
  if (!value) {
    throw navigationError(code, `${code} navigation target unsupported`, ref);
  }
  return builder();
}

function navigationError(code, message, pageReference) {
  const err = new Error(message);
  err.code = code;
  err.body = { code, message, pageReference };
  reportDiagnostic({ code, severity: "warning", message, pageReference });
  return err;
}

function currentPath() {
  return window.location.pathname + window.location.search;
}

function componentPath(componentName) {
  const value = String(componentName || "");
  if (value.includes(":")) {
    return value.split(":").map(enc).join("/");
  }
  const double = value.indexOf("__");
  if (double > 0) {
    return `${enc(value.slice(0, double))}/${enc(value.slice(double + 2))}`;
  }
  return `c/${enc(value)}`;
}

function objectFromQuickAction(apiName) {
  const parts = String(apiName || "").split(".");
  return parts.length > 1 ? parts[0] : "";
}

function actionFromQuickAction(apiName) {
  const parts = String(apiName || "").split(".");
  return parts.length > 1 ? parts.slice(1).join(".") : parts[0];
}

function withState(path, state = {}) {
  const params = new URLSearchParams();
  for (const [key, value] of Object.entries(state || {})) {
    if (value === undefined || value === null || value === "") {
      continue;
    }
    params.set(key, String(value));
  }
  const query = params.toString();
  if (!query) {
    return path;
  }
  const separator = String(path).includes("?") ? "&" : "?";
  return `${path}${separator}${query}`;
}

function enc(value) {
  return encodeURIComponent(String(value || ""));
}
