package apextest

import "testing"

func TestIterableCollectionCastsNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "iterable_collection_casts", "CollectionCastShapesTest")
}
