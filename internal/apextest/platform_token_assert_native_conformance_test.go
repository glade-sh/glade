package apextest

import "testing"

func TestPlatformTokenAssertNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "platform_token_assert", "PlatformTokenAssertTest", []string{"62.0", "67.0"}, 3)
}
