package apextest

import (
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
)

func assertOneShotEntriesHaveNoAuthority(t *testing.T) {
	t.Helper()
	runtimeCacheMu.RLock()
	defer runtimeCacheMu.RUnlock()
	for key, entry := range runtimeCache {
		if entry.patchAuthority != nil {
			t.Fatalf("one-shot runtime entry %v kept a patch authority", key)
		}
	}
}

func TestOneShotRuntimeBuildsNoPatchProof(t *testing.T) {
	_, summary, _ := runOneShotWorkload(t, true)
	if summary != [2]int{1, 1} {
		t.Fatalf("summary = %v", summary)
	}
	assertOneShotEntriesHaveNoAuthority(t)
}

func TestOneShotPerfRuntimeBuildsNoPatchProof(t *testing.T) {
	InvalidateRuntimeCaches()
	t.Cleanup(InvalidateRuntimeCaches)
	fixture := newRuntimeTransitionFixture(t)
	writeFile(t, filepath.Join(fixture.root, "force-app/main/default/classes/OneShotWorkloadTest.cls"), oneShotWorkloadSource)
	index, digests := fixture.fullIndex(t)
	summary := Run(index, Options{SourceDigests: digests, NoDiskCache: true, OneShotProcess: true, PerfCounters: true}).Summary()
	if summary.Total != 1 || summary.Passed != 1 {
		t.Fatalf("summary = %#v", summary)
	}
	assertOneShotEntriesHaveNoAuthority(t)
}

// A one-shot process makes no later transition, so it builds no patch
// authority: the payload, index and source-reference proofs are skipped.
func TestOneShotAuthoritySkipsProofs(t *testing.T) {
	fixture := newRuntimeTransitionFixture(t)
	writeFile(t, filepath.Join(fixture.root, "force-app/main/default/classes/OneShotWorkloadTest.cls"), oneShotWorkloadSource)
	index, digests := fixture.fullIndex(t)
	if authority := newRuntimePatchAuthority(index, ""), digests, &sourceCache{}, runtimeCacheEntry{}, storage.OrgState{}, true); authority != nil {
		t.Fatal("one-shot run built a patch authority")
	}
}
