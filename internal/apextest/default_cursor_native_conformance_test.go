package apextest

import "testing"

func TestDefaultCursorNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "default_cursor", "DefaultCursorObservationTest", []string{"62.0", "67.0"}, 2)
}
