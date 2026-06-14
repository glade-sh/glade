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
		"@salesforce/apex/":            "/lightning/shims/apex/",
		"@salesforce/contentAssetUrl/": "/lightning/shims/contentAssetUrl/",
		"@salesforce/i18n/":            "/lightning/shims/i18n/",
		"@salesforce/label/":           "/lightning/shims/label/",
		"@salesforce/resourceUrl/":     "/lightning/shims/resourceUrl/",
		"@salesforce/schema/":          "/lightning/shims/schema/",
		"@salesforce/user/":            "/lightning/shims/user/",
		"lightning/navigation":         "/lightning/shims/lightning/navigation.js",
		"lightning/uiRecordApi":        "/lightning/shims/lightning/uiRecordApi.js",
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

func ResourceURLModuleJS(url string) string {
	return fmt.Sprintf("export default %q;\n", url)
}

func ContentAssetURLModuleJS(url string) string {
	return fmt.Sprintf("export default %q;\n", url)
}

func NavigationModuleJS() string {
	return `const message = "lightning/navigation is not implemented in the local Lightning runtime";
function unsupported() {
  throw new Error(message);
}
export function CurrentPageReference() {
  return { data: undefined, error: { message } };
}
export function NavigationMixin(Base) {
  return class extends Base {
    [NavigationMixin.Navigate]() {
      unsupported();
    }
    [NavigationMixin.GenerateUrl]() {
      return Promise.reject(new Error(message));
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
  objectApiName: config && config.objectApiName
}));
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
