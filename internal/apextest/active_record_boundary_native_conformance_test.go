package apextest

import "testing"

func TestActiveRecordBoundaryNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "active_record_boundary", "ActiveRowProbeTest", "62.0", "67.0")
}
