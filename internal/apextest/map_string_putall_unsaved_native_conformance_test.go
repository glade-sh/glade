package apextest

import "testing"

func TestMapStringPutAllUnsavedNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "map_string_putall_unsaved", "StringRecordMapProbeTest", "62.0", "67.0")
}
