package storage

import "testing"

// Native describe/type rows: org_commerce_exceptions/org.tsv; plain twin is
// org_commerce_family/raw-plain.jsonl.
func TestPendingOrderSummaryAvailabilityShapes(t *testing.T) {
	for _, tc := range []struct {
		name     string
		features []string
		want     bool
	}{{"plain", nil, false}, {"feature", []string{"OrderManagement"}, true}} {
		t.Run(tc.name, func(t *testing.T) {
			org := NewOrgState()
			EnsureStandardObject(&org, "PendingOrderSummary")
			ApplyProjectOrgShape(&org, tc.features)
			if got := org.AllowsStandardObject("PendingOrderSummary"); got != tc.want {
				t.Errorf("allowed=%t want=%t", got, tc.want)
			}
		})
	}
}
