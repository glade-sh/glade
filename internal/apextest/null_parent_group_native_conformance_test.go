package apextest

import "testing"

func TestNullParentGroupNativeConformance(t *testing.T) {
	runNativeSnapshotConformance(t, "null_parent_group_native.json", "NullParentGroupNative")
}
