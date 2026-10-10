package apextest

import (
	"fmt"
	"testing"
)

func TestRuntimeCacheEvictsLeastRecentlyUsedGeneration(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	key := func(i int) runtimeCacheKey { return runtimeCacheKey(fmt.Sprintf("bound-%02d", i)) }
	store := func(k runtimeCacheKey) []runtimeCacheKey {
		runtimeCacheMu.Lock()
		evicted := storeRuntimeCacheEntryLocked(k, runtimeCacheEntry{})
		runtimeCacheMu.Unlock()
		dropEvictedRuntimeCacheCompanions(evicted)
		return evicted
	}
	for i := 0; i < maxRuntimeCacheEntries; i++ {
		if evicted := store(key(i)); len(evicted) != 0 {
			t.Fatalf("store %d evicted %v below the cap", i, evicted)
		}
		semaDiagnosticsCache[key(i)] = nil
		setupCache[string(key(i))+"|setup|x"] = setupCompileCacheEntry{}
		testCache[string(key(i))+"|tests|x"] = testCompileCacheEntry{}
	}
	// A hit on the oldest generation keeps it; the next oldest goes instead.
	touchRuntimeCacheKey(key(0))
	evicted := store(key(maxRuntimeCacheEntries))
	if len(evicted) != 1 || evicted[0] != key(1) {
		t.Fatalf("evicted %v, want [%s]", evicted, key(1))
	}
	if len(runtimeCache) != maxRuntimeCacheEntries {
		t.Fatalf("runtime cache entries = %d, want %d", len(runtimeCache), maxRuntimeCacheEntries)
	}
	if _, ok := runtimeCache[key(0)]; !ok {
		t.Fatal("recently used generation was evicted")
	}
	if _, ok := semaDiagnosticsCache[key(1)]; ok {
		t.Fatal("evicted generation kept its semantic diagnostics")
	}
	if _, ok := setupCache[string(key(1))+"|setup|x"]; ok {
		t.Fatal("evicted generation kept its setup compile cache")
	}
	if _, ok := testCache[string(key(1))+"|tests|x"]; ok {
		t.Fatal("evicted generation kept its test compile cache")
	}
	if _, ok := testCache[string(key(2))+"|tests|x"]; !ok {
		t.Fatal("retained generation lost its test compile cache")
	}
	// Re-storing a cached key never evicts it or grows the cache.
	if evicted := store(key(maxRuntimeCacheEntries)); len(evicted) != 0 {
		t.Fatalf("re-store evicted %v", evicted)
	}
	if len(runtimeCacheUse) != len(runtimeCache) {
		t.Fatalf("use stamps = %d, entries = %d", len(runtimeCacheUse), len(runtimeCache))
	}
}
