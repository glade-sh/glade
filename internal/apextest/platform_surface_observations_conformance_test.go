package apextest

import "testing"

func TestPlatformSurfaceObservationsNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "platform_surface_observations", "SurfaceObservationTest", []string{"62.0", "67.0", "68.0"}, 6)
}
