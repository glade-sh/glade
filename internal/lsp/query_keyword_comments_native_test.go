package lsp

import "testing"

func TestQueryKeywordCommentsLSPNativeRoutes(t *testing.T) {
	runCapturedSemanticBodyLSP(t, "../sema/testdata/query_keyword_comments")
}
