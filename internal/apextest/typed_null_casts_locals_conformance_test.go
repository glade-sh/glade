package apextest

import "testing"

func TestTypedNullCastsLocalsOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_casts_locals.json", "casts-locals", 12, 72)
}
