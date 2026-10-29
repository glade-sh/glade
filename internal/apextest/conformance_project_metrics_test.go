package apextest

import (
	"reflect"
	"strings"
	"testing"
	"time"
)

// Cache reuse must remove preparation work, rather than accepting a previous
// row result. These are cache-build counts, not Salesforce result counts.
func TestConformanceProjectTemplateBuildAccounting(t *testing.T) {
	cache := newConformanceProjectCache(t)
	start := time.Now()
	for row := 0; row < 5; row++ {
		cache.index(t, "62.0", nil)
	}
	reused := time.Since(start)
	stats := cache.statistics()
	if stats.Requests != 5 || stats.Hits != 4 || stats.Builds != 1 || stats.Evictions != 0 {
		t.Fatalf("unexpected reuse accounting: %+v", stats)
	}
	start = time.Now()
	for row := 0; row < 5; row++ {
		newConformanceProjectCache(t).index(t, "62.0", nil)
	}
	fresh := time.Since(start)
	before := cache.index(t, "62.0", nil)
	for row := 0; row < conformanceProjectCacheLimit; row++ {
		cache.index(t, "62.0", map[string]string{"variant.txt": strings.Repeat("x", row+1)})
	}
	after := cache.statistics()
	if after.Builds != 65 || after.Evictions != 1 || after.Requests != 70 {
		t.Fatalf("unexpected eviction accounting: %+v", after)
	}
	again := cache.index(t, "62.0", nil)
	if reflect.DeepEqual(before.Project.Root, again.Project.Root) {
		t.Fatal("evicted index was not rebuilt")
	}
	t.Logf("project preparation: requests=5 reused-builds=%d fresh-builds=5 reused=%s fresh=%s index-build=%s", stats.Builds, reused, fresh, stats.BuildTime)
}
