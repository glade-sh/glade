package apextest

import "testing"

// NATIVE after-update email template and host formula: org.tsv lines 2-3,
// isolated tests at API 62.0 and 67.0.
func TestFlowAPIEmailUpdateNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "flow_api_email_update", 2, nil)
}
