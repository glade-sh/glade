package apextest

import "testing"

// A type declared in the user's code shadows a System type of the same name
// inside its own scope only. The native matrix covers catch resolution, class
// literals, type names and an unrelated class that still sees the System type.
func TestSystemTypeShadowNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "system_type_shadow", "ShadowProbeTest", nil, "62.0", "67.0")
}
