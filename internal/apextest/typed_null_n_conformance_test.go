package apextest

import "testing"

// Exact owned sources and independent native answers at all three APIs and
// both invocation routes. JSON objects are compared without member ordering.
func TestTypedNullNOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_n.json", "N01-N50", 50, 300)
}
