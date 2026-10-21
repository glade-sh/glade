package apextest

import "testing"

func TestTypedNullTernaryPositionsOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_ternary_positions.json", "ternary-positions", 10, 60)
}
