package apextest

import "testing"

func TestOptionalLookupRestoreNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "optional_lookup_restore", "LookupRestoreProbeTest", "62.0", "67.0")
}
