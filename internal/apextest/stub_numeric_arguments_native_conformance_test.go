package apextest

import "testing"

func TestStubNumericArgumentsNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "stub_numeric_arguments", "StubNumericArgumentsTest", []string{"62.0", "67.0"}, 2)
}
