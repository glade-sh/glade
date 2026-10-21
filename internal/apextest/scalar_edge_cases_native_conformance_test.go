package apextest

import "testing"

func TestScalarEdgeCasesNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "scalar_edge_cases", "ScalarEdgesTest", []string{"62.0", "67.0"}, 8)
}
