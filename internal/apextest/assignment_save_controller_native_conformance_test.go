package apextest

import "testing"

func TestAssignmentSaveStandardControllerNativeConformanceAPIRoutes(t *testing.T) {
	testAssignmentSaveNativeAPIRoutes(t, "testdata/conformance/assignment_save_controller_routes", "named_istest")
}
