package storage

import "testing"

// Native feature-only, setting-only and combined rows: org_commerce_exceptions/org.tsv.
func TestUserLocationAssignmentAvailabilityShapes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{
		{"plain", nil, false}, {"feature-only", []string{"OrderManagement"}, false},
		{"setting-only", []string{"OrderManagementEnabled"}, false},
		{"enabled", []string{"OrderManagement", "OrderManagementEnabled"}, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, "UserLocationAssignment")
			ApplyProjectOrgShape(&org, tc.features)
			if got := org.AllowsStandardObject("UserLocationAssignment"); got != tc.want {
				t.Errorf("allowed=%t want=%t", got, tc.want)
			}
		})
	}
}
