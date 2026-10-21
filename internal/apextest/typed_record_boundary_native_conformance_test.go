package apextest

import "testing"

func TestTypedRecordBoundaryNativeConformance(t *testing.T) {
	runNativeSnapshotConformance(t, "typed_record_boundary_native.json", "TypedRecordBoundaryNative")
}
