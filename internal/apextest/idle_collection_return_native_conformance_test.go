package apextest

import "testing"

func TestIdleCollectionReturnNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "idle_collection_returns", "IdleCollectionProbeTest", "62.0", "67.0")
}
