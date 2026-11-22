package apextest

import (
	"path/filepath"
	"testing"
)

func TestOneShotRuntimeDoesNotPublishPatchProof(t *testing.T) {
	_, summary, _ := runOneShotWorkload(t, true)
	if summary != [2]int{1, 1} {
		t.Fatalf("summary = %v", summary)
	}
	runtimeCacheMu.RLock()
	count := len(runtimeCache)
	runtimeCacheMu.RUnlock()
	if count != 0 {
		t.Fatalf("one-shot runtime published %d cache entries", count)
	}
}

func TestOneShotPerfRuntimeDoesNotPublishPatchProof(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	writeFile(t, filepath.Join(fixture.root, "force-app/main/default/classes/OneShotWorkloadTest.cls"), oneShotWorkloadSource)
	index, digests := fixture.fullIndex(t)
	summary := Run(index, Options{SourceDigests: digests, NoDiskCache: true, OneShotProcess: true, PerfCounters: true}).Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	runtimeCacheMu.RLock()
	count := len(runtimeCache)
	runtimeCacheMu.RUnlock()
	if count != 0 {
		t.Fatalf("one-shot perf runtime published %d cache entries", count)
	}
}
