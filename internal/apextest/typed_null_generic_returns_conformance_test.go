package apextest

import "testing"

func TestTypedNullGenericReturnsOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_generic_returns.json", "generic-returns", 12, 72)
}
