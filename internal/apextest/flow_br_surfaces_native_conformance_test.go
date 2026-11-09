package apextest

import "testing"

// BR() in formulas evaluated outside a stored formula field (org capture
// flow_br_surfaces, API 62.0 and 67.0, anonymous and isTest): BR() is the
// 12-character text _BR_ENCODED_ inside a flow, workflow, default value or
// validation rule formula, and a finished flow formula reads as a line feed.
func TestFlowBRSurfacesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "flow_br_surfaces", "BrProbeTest", compareFormulaObservationLeaves, "62.0", "67.0")
}
