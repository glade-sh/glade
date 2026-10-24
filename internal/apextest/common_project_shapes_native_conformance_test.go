package apextest

import "testing"

func TestCommonProjectShapesNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "common_project_shapes", "CommonProjectShapesTest", []string{"62.0", "67.0"}, 4)
}
