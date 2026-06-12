package lwcbrowser

import (
	"fmt"
	"strings"

	"github.com/glade-sh/glade/internal/resource"
	"github.com/glade-sh/glade/internal/storage"
)

// SalesforceImportMap returns import-map entries for @salesforce/* and lightning/* modules.
func SalesforceImportMap() map[string]string {
	return map[string]string{
		"@salesforce/apex/":        "/lightning/shims/apex/",
		"@salesforce/label/":       "/lightning/shims/label/",
		"@salesforce/schema/":      "/lightning/shims/schema/",
		"@salesforce/resourceUrl/": "/lightning/shims/resourceUrl/",
		"lightning/uiRecordApi":    "/lightning/shims/lightning/uiRecordApi.js",
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

func splitNamespaceQualified(qualified string) (namespace, name string, ok bool) {
	qualified = strings.TrimSpace(qualified)
	dot := strings.LastIndex(qualified, ".")
	if dot <= 0 || dot >= len(qualified)-1 {
		return "", "", false
	}
	return qualified[:dot], qualified[dot+1:], true
}
