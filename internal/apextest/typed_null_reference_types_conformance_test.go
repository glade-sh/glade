package apextest

import "testing"

func TestTypedNullReferenceTypesOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_reference_types.json", "reference-types", 13, 78)
}
