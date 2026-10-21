package apextest

import "testing"

func TestAssetOwnerNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "asset_owner_validation", "AssetOwnerProbeTest", "62.0", "67.0")
}
