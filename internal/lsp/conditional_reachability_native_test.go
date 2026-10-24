package lsp

import "testing"

func TestConditionalReachabilityLSPNativeRoutes(t *testing.T) {
	runCapturedSemanticBodyLSP(t, "../sema/testdata/conditional_reachability")
}
