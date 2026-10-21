package apextest

import "testing"

func TestCollectionBoundaryNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "collection_and_boundary", "BoundarySliceTest", []string{"62.0", "67.0"}, 8)
}
