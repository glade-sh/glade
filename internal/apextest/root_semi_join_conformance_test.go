package apextest

import "testing"

func TestRootSemiJoinNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "root_semi_join", "RootSemiJoinTest", []string{"62.0", "67.0"}, 4)
}
