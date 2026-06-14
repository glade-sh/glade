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
	return map[string]string{
		"@salesforce/apex/":                "/lightning/shims/apex/",
		"@salesforce/contentAssetUrl/":     "/lightning/shims/contentAssetUrl/",
		"@salesforce/i18n/":                "/lightning/shims/i18n/",
		"@salesforce/label/":               "/lightning/shims/label/",
		"@salesforce/resourceUrl/":         "/lightning/shims/resourceUrl/",
		"@salesforce/schema/":              "/lightning/shims/schema/",
		"@salesforce/user/":                "/lightning/shims/user/",
		"lightning/messageService":         "/lightning/shims/lightning/messageService.js",
		"lightning/navigation":             "/lightning/shims/lightning/navigation.js",
		"lightning/platformResourceLoader": "/lightning/shims/lightning/platformResourceLoader.js",
		"lightning/platformShowToastEvent": "/lightning/shims/lightning/platformShowToastEvent.js",
		"lightning/uiRecordApi":            "/lightning/shims/lightning/uiRecordApi.js",
	}
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
		return "export default false;\n"
	default:
		return unsupportedModuleJS("Unsupported @salesforce/user property: " + property)
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

func NavigationModuleJS() string {
	return `function readConfig() {
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
function currentPageReference() {
  return readConfig().pageReference || { type: "standard__namedPage", attributes: { pageName: "home" }, state: {} };
}
function urlFor(pageReference) {
  const ref = pageReference || {};
  const attrs = ref.attributes || {};
  if (ref.type === "standard__recordPage" && attrs.recordId) {
    const objectApiName = attrs.objectApiName || "Record";
    return "/lwc/preview/record/" + encodeURIComponent(objectApiName) + "/" + encodeURIComponent(attrs.recordId);
  }
  if (ref.type === "standard__navItemPage" && attrs.apiName) {
    return "/lwc/preview/tab/" + encodeURIComponent(attrs.apiName);
  }
  if (ref.type === "standard__component" && attrs.componentName) {
    return "/lwc/preview/component/" + String(attrs.componentName).replace(":", "/");
  }
  if (ref.type === "standard__webPage" && attrs.url) {
    return attrs.url;
  }
  if (ref.type === "standard__namedPage" && attrs.pageName === "home") {
    return "/lwc/preview/home";
  }
  return "#";
}
export const CurrentPageReference = class CurrentPageReferenceAdapter {
  constructor(dataCallback) {
    this.dataCallback = dataCallback;
  }
  connect() {
    this.dataCallback(currentPageReference());
  }
  update() {
    this.dataCallback(currentPageReference());
  }
  disconnect() {}
};
export function NavigationMixin(Base) {
	  return class extends Base {
	    [NavigationMixin.Navigate](pageReference) {
	      const url = urlFor(pageReference);
	      if (url && url !== "#") {
	        window.location.assign(url);
	      }
	    }
    [NavigationMixin.GenerateUrl](pageReference) {
      return Promise.resolve(urlFor(pageReference));
    }
  };
}
NavigationMixin.Navigate = Symbol("lightning/navigation.Navigate");
NavigationMixin.GenerateUrl = Symbol("lightning/navigation.GenerateUrl");
export default NavigationMixin;
`
}

func UIRecordAPIModuleJS() string {
	return `import { createFetchWireAdapter, createGetRecordWireAdapter } from "/lightning/shims/core/wire-adapter.js";
export const getRecord = createGetRecordWireAdapter();
export const getObjectInfo = createFetchWireAdapter("/lightning/wire/getObjectInfo", (config) => ({
  objectApiName: objectApiName(config && config.objectApiName)
}));
function objectApiName(value) {
  if (value && typeof value === "object" && value.objectApiName) {
    return value.objectApiName;
  }
  return value;
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
  });
}
export function updateRecord(recordInput) {
  return post("/lightning/wire/updateRecord", {
    fields: recordInput && recordInput.fields || {}
  });
}
export function deleteRecord(recordId) {
  return post("/lightning/wire/deleteRecord", { recordId });
}
export function getRecordNotifyChange() {
  return Promise.resolve();
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
`
}

func ShowToastEventModuleJS() string {
	return `export class ShowToastEvent extends CustomEvent {
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
	return `const channels = new Map();
export const APPLICATION_SCOPE = Symbol("APPLICATION_SCOPE");
export class MessageContext {}
export function createMessageContext() {
  return new MessageContext();
}
export function releaseMessageContext(_context) {}
export function subscribe(_context, channel, listener) {
  const key = String(channel && (channel.name || channel) || "default");
  const bucket = channels.get(key) || new Set();
  bucket.add(listener);
  channels.set(key, bucket);
  return { key, listener };
}
export function unsubscribe(subscription) {
  if (!subscription) {
    return;
  }
  const bucket = channels.get(subscription.key);
  if (bucket) {
    bucket.delete(subscription.listener);
  }
}
export function publish(_context, channel, message) {
  const key = String(channel && (channel.name || channel) || "default");
  const bucket = channels.get(key);
  if (!bucket) {
    return;
  }
  for (const listener of [...bucket]) {
    listener(message);
  }
}
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
