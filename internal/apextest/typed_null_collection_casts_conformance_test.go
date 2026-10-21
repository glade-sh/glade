package apextest

import "testing"

func TestTypedNullCollectionCastsOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_collection_casts.json", "collection-casts", 9, 54)
}
