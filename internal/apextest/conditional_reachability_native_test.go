package apextest

import "testing"

func TestConditionalReachabilityProjectNativeRoutes(t *testing.T) {
	runCapturedSemanticBodyProjects(t, "../sema/testdata/conditional_reachability")
}
