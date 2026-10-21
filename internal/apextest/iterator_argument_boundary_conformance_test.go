package apextest

import "testing"

func TestIteratorArgumentBoundaryOrgConformance(t *testing.T) {
	runNativeObservationConformance(t, "iterator_argument_boundary.json", "IteratorArgumentBoundary", 16)
}
