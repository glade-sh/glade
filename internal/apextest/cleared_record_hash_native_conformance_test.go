package apextest

import "testing"

func TestClearedRecordHashNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "cleared_record_hash", "ClearedHash62Test", "62.0", "67.0", "68.0")
}
