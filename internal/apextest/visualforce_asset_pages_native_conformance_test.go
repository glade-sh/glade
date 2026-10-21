package apextest

import "testing"

func TestVisualforceAssetPagesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "visualforce_asset_pages", "AssetPageProbeTest", "59.0", "67.0")
}
