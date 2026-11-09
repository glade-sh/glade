package project

import (
	"os"
	"path/filepath"
	"testing"
)

// standard_enhanced_commerce_orders captures the setting as the only gate. The
// OrderSaveBehaviorBoth feature alone leaves the fields out, so it must not
// map to the feature. The setting maps whether or not the feature is present.
func TestEnhancedCommerceOrdersSettingNativeRoutes(t *testing.T) {
	for _, variant := range []struct {
		name    string
		enabled bool
	}{{"feature-only", false}, {"setting-only", true}, {"feature-and-setting", true}, {"capitalized-settings", true}} {
		for _, route := range []string{"sfdx-scratch-definition", "cumulusci-scratch-definition"} {
			t.Run(variant.name+"/"+route, func(t *testing.T) {
				root := t.TempDir()
				raw, err := os.ReadFile("../apextest/testdata/conformance/standard_enhanced_commerce_orders/definitions/" + variant.name + ".json")
				if err != nil {
					t.Fatal(err)
				}
				config := "config/project-scratch-def.json"
				if route == "cumulusci-scratch-definition" {
					config = "orgs/dev.json"
					writeFile(t, filepath.Join(root, "cumulusci.yml"), "orgs:\n  scratch:\n    dev:\n      config_file: orgs/dev.json\n")
				}
				writeFile(t, filepath.Join(root, config), string(raw))
				found := false
				for _, feature := range OrgShapeFeatures(root) {
					found = found || feature == "EnhancedCommerceOrders"
				}
				if found != variant.enabled {
					t.Fatalf("features=%v, want EnhancedCommerceOrders=%t", OrgShapeFeatures(root), variant.enabled)
				}
			})
		}
	}
}

// The captured block spelling varies; the key spelling has no such evidence.
func TestEnhancedCommerceOrdersSettingKeyCaseRemainsExact(t *testing.T) {
	for _, block := range []string{"orderSettings", "OrderSettings"} {
		settings := map[string]any{block: map[string]any{"EnableEnhancedCommerceOrders": true}}
		for _, feature := range scratchSettingsFeatures(settings) {
			if feature == "EnhancedCommerceOrders" {
				t.Fatalf("miscased key enabled feature in %s", block)
			}
		}
	}
}
