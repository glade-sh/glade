package apextest

import "testing"

func TestCursorAndNullNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "cursor_and_null", "CursorMapObservationTest", []string{"62.0", "67.0"}, 5)
}
