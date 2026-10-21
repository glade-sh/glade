package apextest

import "testing"

func TestIteratorEndBoundaryNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "iterator_end_boundary", "IteratorEndProbeTest", "62.0", "67.0")
}
