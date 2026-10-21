package apextest

import "testing"

func TestTypedNullCollectionFieldsOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_collection_fields.json", "collection-fields", 12, 72)
}
