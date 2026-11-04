package soql

import "testing"

func TestCase332SparseProjectionAllocations(t *testing.T) {
	org := aggregateTestOrg()
	query, err := Parse("SELECT Name, toLabel(Rating) ratingLabel, FORMAT(AnnualRevenue) formattedRevenue FROM Account WHERE Name = 'not-present'")
	if err != nil {
		t.Fatal(err)
	}
	cache := NewExecutionCache()
	allocations := testing.AllocsPerRun(100, func() {
		result, err := ExecuteWithCache(org, query, cache)
		if err != nil {
			t.Fatal(err)
		}
		if result.Rows != 0 || len(result.Records) != 0 {
			t.Fatalf("unexpected result %#v", result)
		}
	})
	t.Logf("SPARSE_QUERY_ALLOCS %.0f", allocations)
}
