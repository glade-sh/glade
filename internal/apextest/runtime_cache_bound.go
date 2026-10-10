package apextest

import (
	"strings"
	"sync"
)

// maxRuntimeCacheEntries bounds the in-process compiled runtime cache. Each
// entry owns a linked base VM and an org template over the standard schema,
// about 100-150 MB for a small project. A CLI run uses one source generation,
// a runtime patch transition reads the previous generation while it publishes
// the current one, and a watch loop that undoes an edit returns to the
// generation before it. In the measured CLI, test-serve, watch, server,
// playground, testdaemon and gladecli traces, three entries kept the same hits
// as eight. A process that alternates among more than three source generations
// can churn and recompile; a long-lived server, daemon or test process retains
// at most three.
const maxRuntimeCacheEntries = 3

var (
	runtimeCacheUseMu    sync.Mutex
	runtimeCacheUse      = make(map[runtimeCacheKey]uint64)
	runtimeCacheUseClock uint64
)

// touchRuntimeCacheKey records a memory cache hit for least-recently-used
// eviction.
func touchRuntimeCacheKey(key runtimeCacheKey) {
	runtimeCacheUseMu.Lock()
	runtimeCacheUseClock++
	runtimeCacheUse[key] = runtimeCacheUseClock
	runtimeCacheUseMu.Unlock()
}

// storeRuntimeCacheEntryLocked publishes entry while the caller holds
// runtimeCacheMu for writing. It evicts least recently used keys beyond
// maxRuntimeCacheEntries and returns them; the caller passes them to
// dropEvictedRuntimeCacheCompanions after releasing runtimeCacheMu.
func storeRuntimeCacheEntryLocked(key runtimeCacheKey, entry runtimeCacheEntry) []runtimeCacheKey {
	runtimeCache[key] = entry
	runtimeCacheUseMu.Lock()
	defer runtimeCacheUseMu.Unlock()
	for cached := range runtimeCache {
		// Entries written without this helper count as recent, not oldest.
		if _, ok := runtimeCacheUse[cached]; !ok && cached != key {
			runtimeCacheUseClock++
			runtimeCacheUse[cached] = runtimeCacheUseClock
		}
	}
	runtimeCacheUseClock++
	runtimeCacheUse[key] = runtimeCacheUseClock
	var evicted []runtimeCacheKey
	for len(runtimeCache) > maxRuntimeCacheEntries {
		var oldest runtimeCacheKey
		found := false
		for cached := range runtimeCache {
			if cached == key {
				continue
			}
			if !found || runtimeCacheUse[cached] < runtimeCacheUse[oldest] {
				oldest, found = cached, true
			}
		}
		if !found {
			break
		}
		delete(runtimeCache, oldest)
		evicted = append(evicted, oldest)
	}
	for used := range runtimeCacheUse {
		if _, ok := runtimeCache[used]; !ok {
			delete(runtimeCacheUse, used)
		}
	}
	return evicted
}

// dropEvictedRuntimeCacheCompanions releases the memory caches derived from an
// evicted runtime generation. Unlike evictSnapshotCaches it keeps the disk
// cache and the root mapping: the generation is still valid, only cold.
func dropEvictedRuntimeCacheCompanions(keys []runtimeCacheKey) {
	if len(keys) == 0 {
		return
	}
	semaDiagnosticsCacheMu.Lock()
	for _, key := range keys {
		delete(semaDiagnosticsCache, key)
	}
	semaDiagnosticsCacheMu.Unlock()
	prefixes := make([]string, len(keys))
	for i, key := range keys {
		prefixes[i] = string(key) + "|"
	}
	hasEvictedPrefix := func(cacheKey string) bool {
		for _, prefix := range prefixes {
			if strings.HasPrefix(cacheKey, prefix) {
				return true
			}
		}
		return false
	}
	setupCacheMu.Lock()
	for cacheKey := range setupCache {
		if hasEvictedPrefix(cacheKey) {
			delete(setupCache, cacheKey)
		}
	}
	setupCacheMu.Unlock()
	testCacheMu.Lock()
	for cacheKey := range testCache {
		if hasEvictedPrefix(cacheKey) {
			delete(testCache, cacheKey)
		}
	}
	testCacheMu.Unlock()
}

func resetRuntimeCacheUse() {
	runtimeCacheUseMu.Lock()
	runtimeCacheUse = make(map[runtimeCacheKey]uint64)
	runtimeCacheUseMu.Unlock()
}
