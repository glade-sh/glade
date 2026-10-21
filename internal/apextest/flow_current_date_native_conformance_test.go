package apextest

import "testing"

func TestFlowCurrentDateNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "flow_current_date", "FlowDateTest", []string{"62.0", "67.0"}, 3)
}
