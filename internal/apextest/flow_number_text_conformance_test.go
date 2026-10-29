package apextest

import "testing"

func TestFlowNumberTextNativeConformance(t *testing.T) {
	runSyntheticObservationNativeConformance(t, "flow_number_text", "NumberTextProbe", "observeNumber", "NUMBER_TEXT_RESULT", 8, false)
}
