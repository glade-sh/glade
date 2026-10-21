package apextest

import "testing"

func TestTypedNullAmbiguityControlsOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_ambiguity_controls.json", "ambiguity-controls", 5, 30)
}
