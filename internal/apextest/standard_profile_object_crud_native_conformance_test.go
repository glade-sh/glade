package apextest

import (
	"reflect"
	"testing"
)

// Object and field describe flags (accessible, createable, updateable,
// deletable, undeletable) for the Standard User and Standard Platform User
// profiles in a plain Developer org. The administrator opt-out fields
// (Lead.DoNotCall and friends) are hidden for every profile in that org; the
// administrator rows for them (and the read-only AuthProvider administrator row) are retained but not asserted.
func TestStandardProfileObjectCRUDNativeConformance(t *testing.T) {
	adminUnverified := map[string]bool{"Lead.DoNotCall": true, "Lead.HasOptedOutOfEmail": true, "Contact.DoNotCall": true, "AuthProvider": true}
	runInnerTypeNativeConformanceWithComparison(t, "standard_profile_object_crud", "ProfileCrudProbeTest", func(t *testing.T, got, want any) {
		actual, actualOK := got.(map[string]any)
		expected, expectedOK := want.(map[string]any)
		if !actualOK || !expectedOK {
			t.Fatalf("invalid observation: native=%v; Glade=%v", want, got)
		}
		for profile, expectedProfile := range expected {
			expectedFlags, _ := expectedProfile.(map[string]any)
			actualFlags, _ := actual[profile].(map[string]any)
			if expectedFlags == nil || actualFlags == nil {
				t.Errorf("%s: missing observation: native=%v; Glade=%v", profile, expectedProfile, actual[profile])
				continue
			}
			for key, value := range expectedFlags {
				if profile == "admin" && adminUnverified[key] {
					continue
				}
				if !reflect.DeepEqual(actualFlags[key], value) {
					t.Errorf("%s %s: native=%v; Glade=%v", profile, key, value, actualFlags[key])
				}
			}
		}
	}, "62.0", "67.0")
}
