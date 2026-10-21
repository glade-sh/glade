package apextest

import "testing"

func TestStubObjectCollisionNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "stub_object_collision", "StubObjectCollisionProbeTest", []string{"62.0", "67.0"}, 5)
}
