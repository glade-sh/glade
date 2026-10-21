package apextest

import "testing"

func TestJunctionVisibilityNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "junction_visibility", "JunctionVisibilityTest", []string{"62.0", "67.0"}, 1)
}
