package lsp

import "testing"

func TestQueryKeywordLocalsLSPNativeRoutes(t *testing.T) {
	runCapturedSemanticBodyLSP(t, "../sema/testdata/query_keyword_locals")
}
