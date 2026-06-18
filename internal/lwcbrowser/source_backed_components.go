package lwcbrowser

import (
	"fmt"
	"strings"
)

var lightningSourceBackedComponents = map[string]string{
	"badge":                  "badge",
	"breadcrumb":             "breadcrumb",
	"breadcrumbs":            "breadcrumbs",
	"buttongroup":            "buttonGroup",
	"menusubheader":          "menuSubheader",
	"verticalnavigationitem": "verticalNavigationItem",
}

func LightningSourceBackedComponentModuleJS(name string) (string, bool) {
	component, ok := sourceBackedLightningComponentName(name)
	if !ok {
		return "", false
	}
	return fmt.Sprintf(`export { default } from "/lightning/runtime/lightning/source/%[1]s/%[1]s.js";
`, component), true
}

func sourceBackedLightningComponentName(name string) (string, bool) {
	key := normalizeLightningBaseComponentName(strings.TrimSuffix(strings.TrimSpace(name), ".js"))
	component, ok := lightningSourceBackedComponents[key]
	return component, ok
}
