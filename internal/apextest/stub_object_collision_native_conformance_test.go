package apextest

import "testing"

func TestStubObjectCollisionNativeConformance(t *testing.T) {
	// ORG-RUN.txt: sf org assign permset --name CanopyCollisionAccess before each API's observations.
	runNativeObservationProjectAssigned(t, "stub_object_collision", "StubObjectCollisionProbeTest", []string{"62.0", "67.0"}, 5, "CanopyCollisionAccess")
}
