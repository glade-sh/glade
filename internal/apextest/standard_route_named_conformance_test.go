package apextest

import "testing"

func TestStandardRouteNamedNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "standard_anonymous_route", "StandardRouteProbeTest", []string{"62.0", "67.0", "68.0"}, 8)
}
