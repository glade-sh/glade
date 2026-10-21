package apextest

import "testing"

func TestImmediateEventFlowNativeConformance(t *testing.T) {
	runNamedObservationNativeConformance(t, "immediate_event_flow", "ImmediateEventFlowProbeTest", "62.0", "67.0")
}
