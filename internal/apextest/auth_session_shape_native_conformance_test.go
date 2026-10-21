package apextest

import "testing"

func TestAuthSessionShapeNativeConformance(t *testing.T) {
	runStandardEventNativeRows(t, "auth_session_shape")
}
