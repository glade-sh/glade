package apextest

import "testing"

func TestCollectionEdgeCasesNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "collection_edge_cases", "CollectionEdgesTest", []string{"62.0", "67.0"}, 3)
}
