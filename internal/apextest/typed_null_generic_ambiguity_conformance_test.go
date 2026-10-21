package apextest

import "testing"

func TestTypedNullGenericAmbiguityOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_generic_ambiguity.json", "generic-ambiguity", 1, 6)
}
