package apextest

import "testing"

func TestStandardObjectAccessNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "standard_object_access", "ObjectAccessProbeTest", "62.0", "67.0")
}
