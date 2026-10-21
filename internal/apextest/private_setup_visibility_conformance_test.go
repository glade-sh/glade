package apextest

import "testing"

func TestPrivateSetupVisibilityNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "private_setup_visibility", "PrivateSetupVisibilityTest", []string{"62.0", "65.0", "66.0", "67.0", "68.0"}, 1)
}
