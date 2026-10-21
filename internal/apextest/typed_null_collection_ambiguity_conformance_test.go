package apextest

import "testing"

func TestTypedNullCollectionAmbiguityOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_collection_ambiguity.json", "collection-ambiguity", 2, 12)
}
