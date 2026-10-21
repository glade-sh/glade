package apextest

import "testing"

func TestPermissionGroupBackingNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "permission_group_backing", "GroupShapeTest", []string{"62.0", "67.0"}, 2)
}
