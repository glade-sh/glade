package apextest

import "testing"

func TestTypedNullTNOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_tn.json", "TN01-TN09", 9, 54)
}
