package apextest

import "testing"

func TestImmediateEventActionChainNativeConformance(t *testing.T) {
	runNamedObservationNativeConformance(t, "immediate_event_action_chain", "ImmediateChainProbeTest", "62.0", "67.0")
}
