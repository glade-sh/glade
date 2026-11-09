package apextest

import "testing"

func TestNoRowsRecordTypeNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "no_rows_recordtype", "RecordTypeNoRowsProbeTest", "62.0", "67.0")
}
