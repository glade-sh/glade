package apextest

import "testing"

func TestCartOrchestratorRuntimeNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "cart_orchestrator_runtime", "OrchSliceTest", []string{"62.0", "67.0"}, 5)
}
