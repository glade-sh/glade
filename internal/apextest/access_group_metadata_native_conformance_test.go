package apextest

import "testing"

func TestAccessGroupMetadataNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "access_group_metadata", "AccessGroupProbeTest", "62.0", "67.0")
}
