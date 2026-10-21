package apextest

import "testing"

func TestEnclosingJoinHelperNativeConformance(t *testing.T) {
	runQueryCompositionNativeConformance(t, "enclosing_join_helper", "JoinProbeTest", 2)
}
