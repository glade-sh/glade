package sema

import (
	"testing"

	"github.com/glade-sh/glade/internal/typesys"
)

func TestCacheOrgCapturedListRemoval(t *testing.T) {
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			for _, tc := range []struct {
				id, source string
				rejected   bool
			}{
				{"N019", `Cache.Org.get(new List<String>{'local.A42Oracle.A42Bulk'});`, true},
				{"N021", `Cache.Org.remove(new List<String>{'local.A42Oracle.A42Bulk'});`, true},
				{"N001-get", `Cache.Org.get('A42Control');`, false},
				{"N001-remove", `Cache.Org.remove('A42Control');`, false},
				{"N020", `Cache.Org.contains(new Set<String>{'local.A42Oracle.A42Bulk','A42BulkMissing'});`, false},
			} {
				t.Run(tc.id, func(t *testing.T) {
					result := AnalyzeAnonymous(typesys.Index{}, tc.source, api)
					if result.HasErrors() != tc.rejected {
						t.Fatalf("rejected=%t expected=%t: %v", result.HasErrors(), tc.rejected, result.Diagnostics)
					}
				})
			}
		})
	}
}
