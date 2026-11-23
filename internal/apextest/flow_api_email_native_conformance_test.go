package apextest

import "testing"

// NATIVE Flow email template and host formula: org.tsv lines 2-3,
// isolated tests at API 62.0 and 67.0.
func TestFlowAPIEmailNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "flow_api_email", 2, nil)
}
