package storage

import "testing"

// Pins the runtime normalization the scratch-feature registry must not alter:
// only the names below fold case or alias; every other name keeps its spelling
// and loses only a ":arg" suffix. Quotes and OrderManagement are Glade feature
// names, not Salesforce ones, and docs/CONFIG.md says feature names match case
// insensitively, so they fold to their registry spelling like the setup gates.
func TestCanonicalFeatureNameRuntimeNormalization(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"multicurrency", "MultiCurrency"},
		{"FlowSites", "Communities"},
		{"HealthCloud:2", "HealthCloud"},
		{"healthcloudfoo", "HealthCloud"},
		{"AddCustomApps:3", "AddCustomApps"},
		{"B2BCommerce:1", "B2BCommerce"},
		{"CommerceEnabled:1", "CommerceEnabled"},
		{"quotes", "Quotes"},
		{"Quotes:1", "Quotes"},
		{"ordermanagement", "OrderManagement"},
		{"OrderManagement", "OrderManagement"},
	} {
		if got := canonicalFeatureName(tc.in); got != tc.want {
			t.Errorf("canonicalFeatureName(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}
