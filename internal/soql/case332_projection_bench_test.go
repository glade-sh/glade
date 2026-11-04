package soql

import "testing"

// BenchmarkCase332SparseProjection includes validation and projection planning
// in each execution so an empty result measures the whole query path.
func BenchmarkCase332SparseProjection(b *testing.B) {
	org := aggregateTestOrg()
	query, err := Parse("SELECT Name, toLabel(Rating) ratingLabel, FORMAT(AnnualRevenue) formattedRevenue FROM Account WHERE Name = 'not-present'")
	if err != nil {
		b.Fatal(err)
	}
	cache := NewExecutionCache()
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		result, err := ExecuteWithCache(org, query, cache)
		if err != nil {
			b.Fatal(err)
		}
		if result.Rows != 0 || len(result.Records) != 0 {
			b.Fatalf("unexpected result: %#v", result)
		}
	}
}
