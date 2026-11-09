package apextest

import "testing"

func TestDefaultPermissionsV035NativeConformance(t *testing.T) {
	runNativeObservationProject(t, "default_permissions_v035", "PermissionProbeTest", []string{"62.0", "67.0"}, 3)
}
