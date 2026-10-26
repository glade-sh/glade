package apextest

import "testing"

func TestFlowConstantAssignmentAnonymousAndIsTestNativeConformance(t *testing.T) {
	runFlowRecordAssignmentNativeConformance(t, "flow_constant_assignment", "GroveConstProbe", "GroveConstProbeTest", "observeConst", "Grove_Const", "FLOW_CONSTANT_ASSIGNMENT", []string{"K01", "K02", "K03", "K04", "K05"})
}
