package soql

import (
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

// Native empty, LIMIT 0 and singleton observations are retained at
// ../apextest/testdata/conformance/seeded_platform_rows/org.tsv.
func TestPlatformCachePartitionQueriesDoNotInventRows(t *testing.T) {
	for _, setup := range []string{"missing_schema", "empty_schema", "feature"} {
		t.Run(setup, func(t *testing.T) {
			org := storage.NewOrgState()
			if setup == "empty_schema" {
				storage.EnsureStandardObject(&org, "PlatformCachePartition")
			} else if setup == "feature" {
				storage.ApplyOrgShape(&org, []string{"PlatformCache"})
			}
			cache := NewExecutionCache()
			for pass := 0; pass < 2; pass++ {
				for _, source := range []string{
					"SELECT count() FROM PlatformCachePartition",
					"SELECT Id FROM PlatformCachePartition LIMIT 1",
					"SELECT Id FROM PlatformCachePartition LIMIT 0",
				} {
					query, err := Parse(source)
					if err != nil {
						t.Fatal(err)
					}
					got, err := ExecuteWithCache(org, query, cache)
					if err != nil {
						t.Fatal(err)
					}
					if query.Count {
						if len(got.Records) != 1 || got.Records[0].Fields["expr0"].Integer != 0 {
							t.Errorf("pass %d %s: count result=%+v; native has zero", pass, source, got)
						}
					} else if got.Rows != 0 {
						t.Errorf("pass %d %s: rows=%d; native has zero", pass, source, got.Rows)
					}
				}
			}
		})
	}
}
