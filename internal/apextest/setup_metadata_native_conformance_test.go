package apextest

import "testing"

func TestSetupMetadataNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "setup_metadata", "SetupMetadataTest", []string{"62.0", "67.0"}, 3)
}
