package apextest

import "testing"

func TestNoRowsStandardNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "no_rows_standard", "StandardNoRowsProbeTest", "62.0", "67.0")
}
