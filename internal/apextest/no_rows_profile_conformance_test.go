package apextest

import "testing"

func TestNoRowsProfileNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "no_rows_profile", "ProfileNoRowsProbeTest", "62.0", "67.0")
}
