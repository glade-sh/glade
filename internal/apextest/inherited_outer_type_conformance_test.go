package apextest

import "testing"

func TestInheritedOuterTypeNativeConformance(t *testing.T) {
	runQueryCompositionNativeConformance(t, "inherited_outer_type", "DriverProbeTest", 3)
}
