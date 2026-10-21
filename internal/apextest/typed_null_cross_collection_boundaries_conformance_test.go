package apextest

import "testing"

func TestTypedNullCrossCollectionBoundariesOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_cross_collection_boundaries.json", "cross-collection-boundaries", 2, 12)
}
