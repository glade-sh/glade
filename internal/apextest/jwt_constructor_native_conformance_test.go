package apextest

import "testing"

func TestJWTConstructorNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "jwt_constructor", "JWTConstructorProbeTest", []string{"62.0", "67.0"}, 1)
}
