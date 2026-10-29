package storage

import "testing"

// OrderManagement constructor rows and the independent Commerce profile
// establish the two provisioning alternatives; license-only remains absent.
// Native standard_field_policy_combinations/org.tsv:2-12,13-23 and ORG-RUN.txt
// bind the Name boundary, empty constructor and missing-field rejection.
func TestStandardFieldPolicyWebStoreProvisioningRouteTwins(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"baseline", nil, false},
		{"order-management", []string{"OrderManagement"}, true},
		{"commerce-license-only", []string{"B2BCommerce", "Communities"}, false},
		{"commerce-setting", []string{"B2BCommerce", "Communities", "CommerceEnabled"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got := StandardObjectConstructorAvailableForFeatures("WebStore", tc.features); got != tc.want {
				t.Fatalf("native constructor availability=%t; got %t", tc.want, got)
			}
			if tc.want {
				definition := ObjectDefinition{APIName: "WebStore"}
				EnsureStandardObjectFieldsForFeatures(&definition, tc.features)
				if !StandardObjectConstructorFieldAllowedForFeatures("WebStore", "Name", tc.features, definition) {
					t.Fatal("native Name boundary rejected")
				}
				if StandardObjectConstructorFieldAllowedForFeatures("WebStore", "UnknownOwnedField", tc.features, definition) {
					t.Fatal("native missing-constructor field accepted")
				}
			}
		})
	}
}
