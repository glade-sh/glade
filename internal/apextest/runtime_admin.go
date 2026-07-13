package apextest

import "github.com/glade-sh/glade/internal/typesys"

// WarmRuntime builds and caches the project test runtime in-process.
// A persistent test server should call this once at startup so client runs
// skip harness setup.
func WarmRuntime(index typesys.Index) {
	_ = WarmRuntimeWithSourceDigests(index, nil)
}

// WarmRuntimeWithSourceDigests warms the runtime with the exact source digest
// snapshot that produced index.
func WarmRuntimeWithSourceDigests(index typesys.Index, digests *typesys.SourceDigestSet) error {
	sources := newSourceCache()
	_, _, err := runtimeFromIndexWithSourceDigests(index, digests, sources)
	return err
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
