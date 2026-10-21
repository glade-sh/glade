package apextest

import "testing"

func TestDateAndLoopBoundaryNativeConformance(t *testing.T) {
	runNativeProjectRouteCounts(t, "date_and_loop_boundary", "BoundaryProbeTest", []string{"62.0", "67.0"}, 6, 4)
}
