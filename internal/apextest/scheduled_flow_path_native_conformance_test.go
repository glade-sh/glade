package apextest

import "testing"

func TestScheduledFlowPathNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "scheduled_flow_path", "ScheduledPathTest", []string{"62.0", "67.0"}, 2)
}
