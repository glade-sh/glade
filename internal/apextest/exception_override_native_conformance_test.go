package apextest

import "testing"

func TestExceptionOverrideNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "exception_override", "FailureOverrideTest", []string{"62.0", "67.0"}, 1)
}
