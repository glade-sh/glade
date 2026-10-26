package apextest

import "testing"

func TestQueryKeywordLocalsProjectNativeRoutes(t *testing.T) {
	runCapturedSemanticBodyProjects(t, "../sema/testdata/query_keyword_locals")
}
