package apextest

import "testing"

func TestDatedDispatchCacheAnonymousAndIsTestNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "dated_dispatch_cache", "PriceProbeTest", "62.0", "67.0")
}
