package apextest

import "testing"

func TestInheritedInnerSuperclassNativeConformance(t *testing.T) {
	runQueryCompositionNativeConformance(t, "inherited_inner_superclass", "ClauseProbeTest", 4)
}
