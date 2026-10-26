package apextest

import "testing"

func TestCollectionRefreshScopeNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "collection_refresh_scope", "ScopeRefreshProbeTest", "62.0", "67.0")
}
