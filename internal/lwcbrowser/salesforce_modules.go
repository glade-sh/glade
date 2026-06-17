package lwcbrowser

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

// SalesforceImportMap returns import-map entries for @salesforce/* and lightning/* modules.
func SalesforceImportMap() map[string]string {
	imports := map[string]string{
		"@glade/shell/app":                 "/lightning/runtime/shell/app.js",
		"@glade/shell/router":              "/lightning/runtime/shell/router.js",
		"@glade/shell/contextPanel":        "/lightning/runtime/shell/context-panel.js",
		"@glade/shell/diagnostics":         "/lightning/runtime/shell/diagnostics.js",
		"@glade/slds":                      "/lightning/runtime/slds/slds-loader.js",
		"@salesforce/apex":                 "/lightning/shims/core/apex.js",
		"@salesforce/apex/":                "/lightning/shims/apex/",
		"@salesforce/community/":           "/lightning/shims/community/",
		"@salesforce/community/basePath":   "/lightning/shims/community/basePath.js",
		"@salesforce/community/Id":         "/lightning/shims/community/Id.js",
		"@salesforce/contentAssetUrl/":     "/lightning/shims/contentAssetUrl/",
		"@salesforce/i18n/":                "/lightning/shims/i18n/",
		"@salesforce/label/":               "/lightning/shims/label/",
		"@salesforce/messageChannel/":      "/lightning/shims/messageChannel/",
		"@salesforce/resourceUrl/":         "/lightning/shims/resourceUrl/",
		"@salesforce/schema/":              "/lightning/shims/schema/",
		"@salesforce/site/":                "/lightning/shims/site/",
		"@salesforce/site/Id":              "/lightning/shims/site/Id.js",
		"@salesforce/user/":                "/lightning/shims/user/",
		"lightning/":                       "/lightning/shims/lightning/",
		"lightning/actions":                "/lightning/shims/lightning/actions.js",
		"lightning/empApi":                 "/lightning/shims/lightning/empApi.js",
		"lightning/flowSupport":            "/lightning/shims/lightning/flowSupport.js",
		"lightning/messageService":         "/lightning/shims/lightning/messageService.js",
		"lightning/navigation":             "/lightning/shims/lightning/navigation.js",
		"lightning/platformResourceLoader": "/lightning/shims/lightning/platformResourceLoader.js",
		"lightning/platformShowToastEvent": "/lightning/shims/lightning/platformShowToastEvent.js",
		"lightning/platformWorkspaceApi":   "/lightning/shims/lightning/platformWorkspaceApi.js",
		"lightning/refresh":                "/lightning/shims/lightning/refresh.js",
		"lightning/uiLayoutApi":            "/lightning/shims/lightning/uiLayoutApi.js",
		"lightning/uiListApi":              "/lightning/shims/lightning/uiListApi.js",
		"lightning/uiObjectInfoApi":        "/lightning/shims/lightning/uiObjectInfoApi.js",
		"lightning/uiRelatedListApi":       "/lightning/shims/lightning/uiRelatedListApi.js",
		"lightning/uiRecordApi":            "/lightning/shims/lightning/uiRecordApi.js",
	}
	for key, value := range SupportedLightningBaseComponentSpecifiers() {
		imports[key] = value
	}
	return imports
}

func ActionsModuleJS() string {
	return `export class CloseActionScreenEvent extends CustomEvent {
  constructor() {
    super("closeactionscreen", { bubbles: true, composed: true });
  }
}
`
}

func FlowSupportModuleJS() string {
	return `export class FlowAttributeChangeEvent extends CustomEvent {
  constructor(attributeName, value) {
    super("flowattributechange", { bubbles: true, composed: true, detail: { attributeName, value } });
  }
}
function flowNavigationEvent(type) {
  return class extends CustomEvent {
    constructor() {
      super(type, { bubbles: true, composed: true });
    }
  };
}
export const FlowNavigationNextEvent = flowNavigationEvent("flownavigationnext");
export const FlowNavigationBackEvent = flowNavigationEvent("flownavigationback");
export const FlowNavigationPauseEvent = flowNavigationEvent("flownavigationpause");
export const FlowNavigationFinishEvent = flowNavigationEvent("flownavigationfinish");
`
}

func RefreshModuleJS() string {
	return `const handlers = new Map();
const containers = new Map();
export class RefreshEvent extends CustomEvent {
  constructor() {
    super("lightning__refresh", { bubbles: true, composed: true });
  }
}
export function registerRefreshHandler(element, handler) {
  handlers.set(element, handler);
  return { element, handler };
}
export function unregisterRefreshHandler(element) {
  handlers.delete(element && element.element || element);
}
export function registerRefreshContainer(element, callback) {
  containers.set(element, callback);
  return { element, callback };
}
export function unregisterRefreshContainer(element) {
  containers.delete(element && element.element || element);
}
export async function __gladeDispatchRefresh(root) {
  const results = [];
  for (const [element, handler] of handlers) {
    if (!root || root === element || (root.contains && root.contains(element))) {
      results.push(await handler());
    }
  }
  return results;
}
`
}

func EmpAPIModuleJS() string {
	return `const subscriptions = new Map();
const errorHandlers = new Set();
let debug = false;
export function subscribe(channel, replayId, callback) {
  const subscription = { channel, replayId, callback };
  if (!subscriptions.has(channel)) {
    subscriptions.set(channel, new Set());
  }
  subscriptions.get(channel).add(subscription);
  return Promise.resolve(subscription);
}
export function unsubscribe(subscription, callback) {
  const set = subscriptions.get(subscription && subscription.channel);
  if (set) {
    set.delete(subscription);
  }
  if (callback) {
    callback({ successful: true, subscription });
  }
  return Promise.resolve({ successful: true, subscription });
}
export function onError(callback) {
  errorHandlers.add(callback);
}
export function setDebugFlag(flag) {
  debug = Boolean(flag);
}
export function isEmpEnabled() {
  return Promise.resolve(true);
}
export function __gladePublish(channel, payload) {
  for (const subscription of subscriptions.get(channel) || []) {
    subscription.callback(payload);
  }
}
export const __gladeEmpState = { subscriptions, errorHandlers, get debug() { return debug; } };
`
}

func ApexWireModuleJS(className, methodName string) string {
	return fmt.Sprintf(
		`import { createApexWireAdapter } from "/lightning/shims/core/wire-adapter.js";`+
			`export default createApexWireAdapter(%q, %q);`,
		className, methodName,
	)
}

func LabelModuleJS(value string) string {
	return fmt.Sprintf("export default %q;\n", value)
}

func UserModuleJS(property, userID string) string {
	switch property {
	case "Id":
		if strings.TrimSpace(userID) == "" {
			userID = "005000000000001"
		}
		return defaultExportJS(userID)
	case "isGuest":
		return `import { readCommunityContext } from "/lightning/runtime/shims/community.js";
function readGuest() {
  return Boolean(readCommunityContext().guest);
}
export default readGuest();
`
	default:
		return unsupportedModuleJS("Unsupported @salesforce/user property: " + property)
	}
}

func CommunityModuleJS(property string) string {
	switch property {
	case "basePath":
		return `import { readCommunityValue } from "/lightning/runtime/shims/community.js";
export default readCommunityValue("basePath", "/s");
`
	case "Id":
		return `import { readCommunityValue } from "/lightning/runtime/shims/community.js";
export default readCommunityValue("networkId", "");
`
	default:
		return unsupportedModuleJS("Unsupported @salesforce/community property: " + property)
	}
}

func SiteModuleJS(property string) string {
	switch property {
	case "Id":
		return `import { readSiteId } from "/lightning/runtime/shims/site.js";
export default readSiteId();
`
	default:
		return unsupportedModuleJS("Unsupported @salesforce/site property: " + property)
	}
}

func I18nModuleJS(property string) string {
	values := map[string]any{
		"currency":                 "USD",
		"dateTime.shortDateFormat": "M/d/yyyy",
		"dir":                      "ltr",
		"firstDayOfWeek":           1,
		"isEasternNameStyle":       false,
		"lang":                     "en-US",
		"locale":                   "en_US",
		"number.currencySymbol":    "$",
		"number.decimalSeparator":  ".",
		"number.groupingSeparator": ",",
		"timeZone":                 "UTC",
	}
	value, ok := values[property]
	if !ok {
		return unsupportedModuleJS("Unsupported @salesforce/i18n property: " + property)
	}
	return defaultExportJS(value)
}

func SchemaFieldModuleJS(objectName, fieldName string) string {
	return fmt.Sprintf(`const token = {
  fieldApiName: %q,
  objectApiName: %q,
  toString() { return %q; },
};
export default token;
`, fieldName, objectName, objectName+"."+fieldName)
}

func SchemaObjectModuleJS(objectName string) string {
	return fmt.Sprintf(`const token = {
  objectApiName: %q,
  toString() { return %q; },
};
export default token;
`, objectName, objectName)
}

func ResourceURLModuleJS(url string) string {
	return fmt.Sprintf("export default %q;\n", url)
}

func ContentAssetURLModuleJS(url string) string {
	return fmt.Sprintf("export default %q;\n", url)
}

func MessageChannelModuleJS(name string) string {
	name = strings.TrimSuffix(strings.TrimSpace(name), ".js")
	return fmt.Sprintf(`const channel = {
  name: %q,
  messageChannelName: %q,
  toString() { return %q; },
};
export default channel;
`, name, name, name)
}

func NavigationModuleJS() string {
	return `import {
  CurrentPageReferenceAdapter,
  generateUrl,
  navigate,
} from "/lightning/runtime/shell/navigation-service.js";

export const supportedPageReferenceTypes = [
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
];
export const navigationDiagnosticCodes = ["GLADELWC040", "GLADELWC041", "GLADELWC042", "GLADELWC103"];
export const CurrentPageReference = CurrentPageReferenceAdapter;
export function NavigationMixin(Base) {
  return class extends Base {
    [NavigationMixin.Navigate](pageReference) {
      navigate(pageReference).catch(() => undefined);
    }
    [NavigationMixin.GenerateUrl](pageReference) {
      return generateUrl(pageReference);
    }
  };
}
NavigationMixin.Navigate = Symbol("lightning/navigation.Navigate");
NavigationMixin.GenerateUrl = Symbol("lightning/navigation.GenerateUrl");
export default NavigationMixin;
`
}

func PlatformWorkspaceAPIModuleJS() string {
	return `const DIAGNOSTIC_CODE = "GLADELWC072";

function readWorkbench() {
  const node = document.getElementById("glade-lwc-workbench");
  if (!node) {
    return {};
  }
  try {
    return JSON.parse(node.textContent || "{}");
  } catch (_err) {
    return {};
  }
}

function activeTab() {
  const model = readWorkbench();
  const active = model.active || {};
  const context = active.context || {};
  const label = context.tabName || context.pageName || context.componentName || model.activeRoute || "Local";
  return {
    tabId: model.activeRoute || "local",
    url: model.activeRoute || "/lwc",
    title: label,
    label,
    icon: "utility:preview",
    customTitle: label,
    highlighted: false,
    closeable: false,
    workspaceTab: true,
    gladeDiagnostic: DIAGNOSTIC_CODE,
  };
}

export async function getFocusedTabInfo() {
  return activeTab();
}

export async function getAllTabInfo() {
  return [activeTab()];
}

export async function setTabLabel(tabId, label) {
  const tab = activeTab();
  tab.tabId = tabId || tab.tabId;
  tab.label = label || tab.label;
  tab.title = tab.label;
  tab.customTitle = tab.label;
  return tab;
}

export async function setTabIcon(tabId, icon) {
  const tab = activeTab();
  tab.tabId = tabId || tab.tabId;
  tab.icon = icon || tab.icon;
  return tab;
}

export async function isConsoleNavigation() {
  return (readWorkbench().mode || "") === "console";
}

export const workspaceDiagnosticCodes = [DIAGNOSTIC_CODE];
`
}

func UIRecordAPIModuleJS() string {
	return `import { createFetchWireAdapter, createGetRecordWireAdapter } from "/lightning/shims/core/wire-adapter.js";
import {
  getRecordNotifyChange,
  notifyRecordUpdateAvailable,
  refreshApex,
} from "/lightning/shims/core/lds-cache.mjs";
export { getRecordNotifyChange, notifyRecordUpdateAvailable, refreshApex };
export const getRecord = createGetRecordWireAdapter();
export const getRecords = createFetchWireAdapter("/lightning/wire/getRecords", (config) => ({
  records: (config && config.records || []).map((record) => ({
    recordIds: record && record.recordIds || [],
    fields: normalizeFields(record && record.fields),
    optionalFields: normalizeFields(record && record.optionalFields)
  }))
}));
export const getObjectInfo = createFetchWireAdapter("/lightning/wire/getObjectInfo", (config) => ({
  objectApiName: objectApiName(config && config.objectApiName)
}));
export const getObjectInfos = createFetchWireAdapter("/lightning/wire/getObjectInfos", (config) => ({
  objectApiNames: (config && config.objectApiNames || []).map(objectApiName)
}));
export const getRecordCreateDefaults = createFetchWireAdapter("/lightning/wire/getRecordCreateDefaults", (config) => ({
  objectApiName: objectApiName(config && config.objectApiName),
  recordTypeId: config && config.recordTypeId,
  optionalFields: normalizeFields(config && config.optionalFields),
  formFactor: config && config.formFactor
}));
export const getPicklistValues = createFetchWireAdapter("/lightning/wire/getPicklistValues", (config) => ({
  objectApiName: objectApiName(config && config.objectApiName),
  fieldApiName: fieldApiName(config && config.fieldApiName),
  recordTypeId: config && config.recordTypeId
}));
export const getPicklistValuesByRecordType = createFetchWireAdapter("/lightning/wire/getPicklistValuesByRecordType", (config) => ({
  objectApiName: objectApiName(config && config.objectApiName),
  recordTypeId: config && config.recordTypeId
}));
export const getRelatedListRecords = createFetchWireAdapter("/lightning/wire/getRelatedListRecords", (config) => ({
  parentRecordId: config && config.parentRecordId,
  relatedListId: config && config.relatedListId,
  fields: normalizeFields(config && config.fields)
}));
export const getListUi = class GetListUiUnsupportedAdapter {
  constructor(dataCallback) {
    this.dataCallback = dataCallback;
  }
  connect() {}
  disconnect() {}
  update() {
    this.dataCallback({
      data: undefined,
      error: {
        code: "GLADELWC050",
        message: "GLADELWC050 getListUi unsupported locally; use getRelatedListRecords or local SOQL-backed Apex"
      }
    });
  }
};
function objectApiName(value) {
  if (value && typeof value === "object" && value.objectApiName) {
    return value.objectApiName;
  }
  return value;
}
function fieldApiName(value) {
  if (value && typeof value === "object") {
    if (value.fieldApiName && value.objectApiName) {
      return value.objectApiName + "." + value.fieldApiName;
    }
    return value.fieldApiName || "";
  }
  return value || "";
}
function normalizeFields(fields) {
  return (fields || []).map((field) => {
    if (field && typeof field === "object") {
      return field.objectApiName && field.fieldApiName ? field.objectApiName + "." + field.fieldApiName : field.fieldApiName;
    }
    return String(field);
  });
}
function post(endpoint, body) {
  return fetch(endpoint, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(body || {})
  }).then((response) => response.json()).then((result) => {
    if (result && result.error) {
      const err = new Error(result.error.message || "Lightning Data Service request failed");
      err.body = result.error;
      throw err;
    }
    return result && result.data;
  });
}
export function createRecord(recordInput) {
  return post("/lightning/wire/createRecord", {
    apiName: recordInput && (recordInput.apiName || recordInput.objectApiName),
    fields: recordInput && recordInput.fields || {}
  }).then((data) => notifyRecordUpdateAvailable(notificationItems(data)).then(() => data));
}
export function updateRecord(recordInput) {
  const recordId = recordInput && recordInput.fields && recordInput.fields.Id;
  return post("/lightning/wire/updateRecord", {
    fields: recordInput && recordInput.fields || {}
  }).then((data) => notifyRecordUpdateAvailable(notificationItems(data, recordId)).then(() => data));
}
export function deleteRecord(recordId) {
  return post("/lightning/wire/deleteRecord", { recordId })
    .then((data) => notifyRecordUpdateAvailable(notificationItems(data, recordId)).then(() => data));
}
export function generateRecordInputForCreate(record, objectInfo) {
  const fields = recordFields(record, objectInfo, "createable");
  delete fields.Id;
  return {
    apiName: record && (record.apiName || record.objectApiName),
    fields
  };
}
export function generateRecordInputForUpdate(record, objectInfo) {
  const fields = recordFields(record, objectInfo, "updateable");
  const id = record && (record.id || record.recordId || fieldValue(record.fields && record.fields.Id));
  if (id !== undefined && id !== null) {
    fields.Id = id;
  }
  return { fields };
}
export function createRecordInputFilteredByEditedFields(recordInput, originalRecord) {
  const sourceFields = recordInput && recordInput.fields || {};
  const out = {};
  for (const [name, value] of Object.entries(sourceFields)) {
    if (name === "Id") {
      out[name] = value;
      continue;
    }
    const original = originalRecord && originalRecord.fields && originalRecord.fields[name];
    if (!sameValue(value, fieldValue(original))) {
      out[name] = value;
    }
  }
  return Object.assign({}, recordInput || {}, { fields: out });
}
export function getFieldValue(record, field) {
  const name = typeof field === "string" ? field.split(".").pop() : field && field.fieldApiName;
  const value = record && record.fields && record.fields[name];
  return value ? value.value : undefined;
}
export function getFieldDisplayValue(record, field) {
  const name = typeof field === "string" ? field.split(".").pop() : field && field.fieldApiName;
  const value = record && record.fields && record.fields[name];
  return value ? value.displayValue : undefined;
}
function recordFields(record, objectInfo, accessProperty) {
  const fields = {};
  const source = record && record.fields || {};
  for (const [name, wrapped] of Object.entries(source)) {
    if (name === "Id" && accessProperty === "createable") {
      continue;
    }
    if (!fieldAllows(objectInfo, name, accessProperty)) {
      continue;
    }
    const value = fieldValue(wrapped);
    if (!recordInputValueSupported(value)) {
      continue;
    }
    fields[name] = value;
  }
  return fields;
}
function fieldAllows(objectInfo, name, accessProperty) {
  const fields = objectInfo && objectInfo.fields || {};
  const field = fields[name];
  if (!field) {
    return true;
  }
  return field[accessProperty] !== false;
}
function fieldValue(value) {
  if (value && typeof value === "object" && Object.prototype.hasOwnProperty.call(value, "value")) {
    return value.value;
  }
  return value;
}
function sameValue(left, right) {
  return JSON.stringify(left) === JSON.stringify(right);
}
function recordInputValueSupported(value) {
  return value === null || value === undefined || typeof value !== "object" || Array.isArray(value);
}
function notificationItems(record, fallbackId) {
  const ids = new Set();
  collectId(ids, fallbackId);
  collectId(ids, record && record.id);
  for (const field of Object.values(record && record.fields || {})) {
    collectId(ids, field && field.value);
  }
  return Array.from(ids).map((recordId) => ({ recordId }));
}
function collectId(ids, value) {
  if (typeof value === "string" && /^[a-zA-Z0-9]{15,18}$/.test(value)) {
    ids.add(value);
  }
}
	`
}

func UIListAPIModuleJS() string {
	return `export const getListUi = class GetListUiUnsupportedAdapter {
  constructor(dataCallback) {
    this.dataCallback = dataCallback;
  }
  connect() {}
  disconnect() {}
  update() {
    this.dataCallback({
      data: undefined,
      error: {
        code: "GLADELWC050",
        message: "GLADELWC050 getListUi unsupported locally; use getRelatedListRecords or local SOQL-backed Apex"
      }
    });
  }
};
`
}

func UILayoutAPIModuleJS() string {
	return `import { createFetchWireAdapter } from "/lightning/shims/core/wire-adapter.js";
export const getLayout = createFetchWireAdapter("/lightning/wire/getLayout", (config) => ({
  objectApiName: objectApiName(config && config.objectApiName),
  recordTypeId: config && config.recordTypeId,
  layoutType: config && config.layoutType,
  mode: config && config.mode,
  formFactor: config && config.formFactor
}));
function objectApiName(value) {
  if (value && typeof value === "object" && value.objectApiName) {
    return value.objectApiName;
  }
  return value;
}
	`
}

func UIObjectInfoAPIModuleJS() string {
	return `import { createFetchWireAdapter } from "/lightning/shims/core/wire-adapter.js";
export const getObjectInfo = createFetchWireAdapter("/lightning/wire/getObjectInfo", (config) => ({
  objectApiName: objectApiName(config && config.objectApiName)
}));
export const getObjectInfos = createFetchWireAdapter("/lightning/wire/getObjectInfos", (config) => ({
  objectApiNames: (config && config.objectApiNames || []).map(objectApiName)
}));
export const getPicklistValues = createFetchWireAdapter("/lightning/wire/getPicklistValues", (config) => ({
  objectApiName: objectApiName(config && config.objectApiName),
  fieldApiName: fieldApiName(config && config.fieldApiName),
  recordTypeId: config && config.recordTypeId
}));
export const getPicklistValuesByRecordType = createFetchWireAdapter("/lightning/wire/getPicklistValuesByRecordType", (config) => ({
  objectApiName: objectApiName(config && config.objectApiName),
  recordTypeId: config && config.recordTypeId
}));
function objectApiName(value) {
  if (value && typeof value === "object" && value.objectApiName) {
    return value.objectApiName;
  }
  return value;
}
function fieldApiName(value) {
  if (value && typeof value === "object") {
    if (value.fieldApiName && value.objectApiName) {
      return value.objectApiName + "." + value.fieldApiName;
    }
    return value.fieldApiName || "";
  }
  return value || "";
}
	`
}

func UIRelatedListAPIModuleJS() string {
	return `import { createFetchWireAdapter } from "/lightning/shims/core/wire-adapter.js";
export const getRelatedListRecords = createFetchWireAdapter("/lightning/wire/getRelatedListRecords", (config) => ({
  parentRecordId: config && config.parentRecordId,
  relatedListId: config && config.relatedListId,
  fields: normalizeFields(config && config.fields),
  optionalFields: normalizeFields(config && config.optionalFields),
  sortBy: normalizeFields(config && config.sortBy),
  pageSize: config && config.pageSize,
  pageToken: config && config.pageToken
}));
function normalizeFields(fields) {
  return (fields || []).map((field) => {
    if (field && typeof field === "object") {
      return field.objectApiName && field.fieldApiName ? field.objectApiName + "." + field.fieldApiName : field.fieldApiName;
    }
    return String(field);
  });
}
`
}

func ShowToastEventModuleJS() string {
	return `import { recordToast } from "/lightning/runtime/shell/toast-service.js";
export { recordToast };
export class ShowToastEvent extends CustomEvent {
  constructor(detail = {}) {
    super("lightning__showtoast", { bubbles: true, composed: true, cancelable: true, detail });
  }
}
export default ShowToastEvent;
`
}

func PlatformResourceLoaderModuleJS() string {
	return `function appendOnce(tag, attr, url) {
  if (!url) {
    return Promise.reject(new Error("resource URL is required"));
  }
  const selector = tag + "[" + attr + "=\"" + url + "\"]";
  if (document.querySelector(selector)) {
    return Promise.resolve();
  }
  return new Promise((resolve, reject) => {
    const el = document.createElement(tag);
    el[attr] = url;
    el.onload = () => resolve();
    el.onerror = () => reject(new Error("failed to load resource: " + url));
    document.head.appendChild(el);
  });
}
export function loadScript(_self, url) {
  return appendOnce("script", "src", url);
}
export function loadStyle(_self, url) {
  const selector = "link[href=\"" + url + "\"]";
  if (document.querySelector(selector)) {
    return Promise.resolve();
  }
  return new Promise((resolve, reject) => {
    const el = document.createElement("link");
    el.rel = "stylesheet";
    el.href = url;
    el.onload = () => resolve();
    el.onerror = () => reject(new Error("failed to load resource: " + url));
    document.head.appendChild(el);
  });
}
`
}

func MessageServiceModuleJS() string {
	return `export {
  APPLICATION_SCOPE,
  MessageContext,
  createMessageContext,
  releaseMessageContext,
  subscribe,
  unsubscribe,
  publish,
} from "/lightning/runtime/shell/message-service.js";
`
}

func ResolveLabelValue(org *storage.OrgState, qualified string) (string, bool) {
	namespace, name, ok := splitNamespaceQualified(qualified)
	if !ok {
		return "", false
	}
	orgNamespace := ""
	if org != nil {
		orgNamespace = org.Namespace
	}
	var registry storage.MetadataRegistry
	if org != nil {
		registry = org.Metadata
	}
	value, status := resource.ResolveLabel(registry, orgNamespace, namespace, name)
	switch status {
	case resource.LabelLookupResolved, resource.LabelLookupPlatformFallback, resource.LabelLookupManagedNamespaceFallback:
		return value, true
	default:
		return "", false
	}
}

func ParseSchemaFieldToken(qualified string) (objectName, fieldName string, ok bool) {
	qualified = strings.TrimSpace(qualified)
	dot := strings.LastIndex(qualified, ".")
	if dot <= 0 || dot >= len(qualified)-1 {
		return "", "", false
	}
	return qualified[:dot], qualified[dot+1:], true
}

func ParseSchemaObjectToken(qualified string) (objectName string, ok bool) {
	qualified = strings.TrimSpace(qualified)
	if qualified == "" || strings.Contains(qualified, ".") || strings.Contains(qualified, "/") {
		return "", false
	}
	return qualified, true
}

func ParseApexWireToken(qualified string) (className, methodName string, ok bool) {
	qualified = strings.TrimSpace(qualified)
	dot := strings.LastIndex(qualified, ".")
	if dot <= 0 || dot >= len(qualified)-1 {
		return "", "", false
	}
	return qualified[:dot], qualified[dot+1:], true
}

func defaultExportJS(value any) string {
	raw, err := json.Marshal(value)
	if err != nil {
		raw = []byte("null")
	}
	return "export default " + string(raw) + ";\n"
}

func unsupportedModuleJS(message string) string {
	raw, err := json.Marshal(message)
	if err != nil {
		raw = []byte(`"Unsupported Salesforce module"`)
	}
	return "throw new Error(" + string(raw) + ");\nexport default undefined;\n"
}

func splitNamespaceQualified(qualified string) (namespace, name string, ok bool) {
	qualified = strings.TrimSpace(qualified)
	dot := strings.LastIndex(qualified, ".")
	if dot <= 0 || dot >= len(qualified)-1 {
		return "", "", false
	}
	return qualified[:dot], qualified[dot+1:], true
}
