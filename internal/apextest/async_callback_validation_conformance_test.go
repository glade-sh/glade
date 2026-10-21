package apextest

import "testing"

func TestAsyncCallbackValidationNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "async_callback_validation", "AsyncValidationTest", []string{"61.0", "62.0", "63.0", "64.0", "65.0", "66.0", "67.0", "68.0"}, 6)
}
