package apextest

import "testing"

// Record-triggered flow routing (org capture flow_assign_routes, API 62.0 and
// 67.0, anonymous and isTest): assignments ahead of a routed Decision, in
// sequence, inside a loop and after branches merge back all apply together
// with the chosen outcome; a Decision reads values an earlier Assignment set;
// after-save flows read stored and cross-object formula fields; custom error
// text finishes its formula merge fields.
func TestFlowAssignRoutesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "flow_assign_routes", "FasgProbeTest", nil, "62.0", "67.0")
}
