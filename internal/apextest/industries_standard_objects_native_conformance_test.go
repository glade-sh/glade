package apextest

import "testing"

// Owned synthetic capture: Account child relationships contributed by industries
// objects, and a calculated standard field defaulted on insert.
func TestIndustriesStandardObjectsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "industries_standard_objects", "IndustriesProbeTest", "62.0", "67.0")
}
