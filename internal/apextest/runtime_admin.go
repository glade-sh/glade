package apextest

import "github.com/glade-sh/glade/internal/typesys"

// WarmRuntime builds and caches the project test runtime in-process.
// A persistent test server should call this once at startup so client runs
// skip harness setup.
func WarmRuntime(index typesys.Index) {
	sources := newSourceCache()
	runtimeFromIndex(index, sources)
}

// InvalidateRuntimeCaches drops compiled runtime and test caches. Call after
// incremental index updates when source files change.
func InvalidateRuntimeCaches() {
	runtimeCacheMu.Lock()
	runtimeCache = make(map[runtimeCacheKey]runtimeCacheEntry)
	runtimeCacheMu.Unlock()

	setupCacheMu.Lock()
	setupCache = make(map[string]setupCompileCacheEntry)
	setupCacheMu.Unlock()

	testCacheMu.Lock()
	testCache = make(map[string]testCompileCacheEntry)
	testCacheMu.Unlock()
}
