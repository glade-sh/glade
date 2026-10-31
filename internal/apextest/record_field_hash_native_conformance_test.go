package apextest

import "testing"

func TestRecordFieldHashNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "record_field_hash", "FieldHash62Test", "62.0", "67.0", "68.0")
}
