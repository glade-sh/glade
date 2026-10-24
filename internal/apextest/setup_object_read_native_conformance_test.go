package apextest

import "testing"

func TestSetupObjectReadNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "setup_object_read", "MeadowSetupProbeTest", "62.0", "67.0")
}

func TestSetupObjectDirectReadNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "setup_object_direct_read", "MeadowDirectProbeTest", "62.0", "67.0")
}
