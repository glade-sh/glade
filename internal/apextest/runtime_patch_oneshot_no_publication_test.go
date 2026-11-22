package apextest

import "testing"

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
