package apextest

import "testing"

func TestStandardSharingCase159NativeConformance(t *testing.T) {
	runNativeObservationProject(t, "case159_sharing", "Case159SharingTest", []string{"62.0", "67.0", "68.0"}, 2)
}
