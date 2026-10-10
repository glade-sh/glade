package vm

import (
	"errors"
	"strings"
	"testing"
)

func TestExecSOSLUpdateViewStatRemainsUnsupportedForQueryAndFindAPI67(t *testing.T) {
	for _, tc := range []struct {
		name   string
		source string
	}{
		{
			name: "Search.query",
			source: `List<List<SObject>> results = Search.query(
				'FIND {GladeClauseProbe} RETURNING KnowledgeArticleVersion(Id) UPDATE VIEWSTAT');`,
		},
		{
			name: "Search.find",
			source: `Search.SearchResults results = Search.find(
				'FIND {GladeClauseProbe} RETURNING KnowledgeArticleVersion(Id) UPDATE VIEWSTAT');`,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			program, err := CompileAnonymousWithOptions(tc.source, CompileOptions{APIVersion: "67.0"})
			if err != nil {
				t.Fatalf("compile API 67.0 source: %v", err)
			}
			if program.APIVersion != "67.0" {
				t.Fatalf("compiled API version = %q, want 67.0", program.APIVersion)
			}

			_, err = New(nil).Execute(program)
			var runtimeErr *RuntimeError
			const want = "SOSL UPDATE VIEWSTAT hosted search analytics"
			if !errors.As(err, &runtimeErr) || runtimeErr.Type != "UnsupportedFeature" || !strings.Contains(runtimeErr.Message, want) {
				t.Fatalf("execution error = %#v, want UnsupportedFeature containing %q", err, want)
			}
		})
	}
}
