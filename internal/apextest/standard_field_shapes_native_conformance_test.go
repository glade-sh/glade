package apextest

import "testing"

func TestStandardFieldShapesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "standard_field_shapes", "StandardFieldShapesProbeTest", "62.0", "67.0")
}
