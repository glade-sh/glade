package apextest

import "testing"

func TestTypedNullTransportBoundariesOrgConformance(t *testing.T) {
	runTypedNullConformance(t, "typed_null_transport_boundaries.json", "transport-controls", 4, 18)
}
