package apextest

import "testing"

func TestSetupRunAsVisibilityNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "setup_runas_visibility", "SetupRunAsVisibilityTest", []string{"62.0", "66.0", "67.0"}, 10)
}
