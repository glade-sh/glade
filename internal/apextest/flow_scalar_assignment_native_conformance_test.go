package apextest

import "testing"

func TestFlowScalarAssignmentNativeConformance(t *testing.T) {
	runFlowRecordAssignmentNativeConformance(t, "flow_scalar_assignment", "GroveScalarProbe", "GroveScalarProbeTest", "observeScalar", "Grove_Scalar", "FLOW_SCALAR_ASSIGNMENT", []string{"A01", "A02", "expressions null Boolean and control flow", "A04", "A05", "A06", "integerLong", "Double and Decimal", "A09", "String", "collections"})
}
