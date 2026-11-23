package apextest

import "testing"

// NATIVE after-update email formula with path: org.tsv lines 2-3,
// isolated tests at API 62.0 and 67.0.
func TestFlowAPIEmailUpdatePathNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "flow_api_email_update_path", 2, nil)
}
