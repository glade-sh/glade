package apextest

import "testing"

func TestJWTAlgorithmBoundaryNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "jwt_algorithm_boundary", "JwtAlgorithmProbeTest", "62.0", "67.0")
}
