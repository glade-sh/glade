package apextest

import "testing"

func TestQueryKeywordCommentsProjectNativeRoutes(t *testing.T) {
	runCapturedSemanticBodyProjects(t, "../sema/testdata/query_keyword_comments")
}
