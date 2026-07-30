package apextest

import (
	"context"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/typesys"
)

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

// WarmRuntimeWithBuildArtifacts warms both the verified semantic result and
// compiled runtime from one captured generation. Persistent test servers use
// this path so their first request performs neither build.
func WarmRuntimeWithBuildArtifacts(ctx context.Context, index typesys.Index, artifacts *typesys.BuildArtifacts) error {
	if ctx == nil {
		ctx = context.Background()
	}
	if err := validateBuildArtifacts(index, artifacts); err != nil {
		return err
	}
	sources := newSourceCache()
	if err := sources.seedBuildArtifacts(index, artifacts); err != nil {
		return err
	}
	digests, err := authoritativeRuntimeSourceDigests(index, artifacts.SourceDigests)
	if err != nil {
		return err
	}
	generation, err := prepareRuntimeGeneration(index, digests, sources)
	if err != nil {
		return err
	}
	if err := semanticCompileErrorWithHooks(ctx, index, artifacts, digests, sources, generation, semanticGateHooks{}, true); err != nil {
		return err
	}
	_, _, err = runtimeFromIndexWithPreparedGenerationProjected(index, digests, sources, &generation, cloneRuntimeCacheEntryChecked, true)
	return err
}

// InvalidateRuntimeCaches drops compiled runtime and test caches. Call after
// incremental index updates when source files change.
func InvalidateRuntimeCaches() {
	semanticResults.Reset()

	runtimePatchTransitionFlights.reset(func() {
		runtimeCacheMu.Lock()
		runtimeCache = make(map[runtimeCacheKey]runtimeCacheEntry)
		runtimeCacheMu.Unlock()
	})
	runtimeCacheRootMu.Lock()
	runtimeCacheRoots = make(map[runtimeCacheKey]string)
	runtimeCacheRootMu.Unlock()

	setupCacheMu.Lock()
	setupCache = make(map[string]setupCompileCacheEntry)
	setupCacheMu.Unlock()

	testCacheMu.Lock()
	testCache = make(map[string]testCompileCacheEntry)
	testCacheMu.Unlock()

	semaDiagnosticsCacheMu.Lock()
	semaDiagnosticsCache = make(map[runtimeCacheKey][]diagnostic.Diagnostic)
	semaDiagnosticsCacheMu.Unlock()
}

// WaitForRuntimeCacheWork drains shared semantic and transition computations
// after callers have stopped admitting new work. Test-daemon shutdown uses
// this to avoid leaving detached leader goroutines behind after cancellation.
func WaitForRuntimeCacheWork() {
	runtimePatchTransitionFlights.Wait()
	semanticResults.Wait()
}

// WaitForSemanticCacheWork is retained for internal callers compiled against
// the earlier lifecycle hook. It now drains every detached runtime-cache task.
func WaitForSemanticCacheWork() {
	WaitForRuntimeCacheWork()
}
