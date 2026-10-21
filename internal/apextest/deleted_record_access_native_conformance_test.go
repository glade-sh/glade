package apextest

import "testing"

func TestDeletedRecordAccessNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "deleted_record_access", "DeletedAccessProbeTest", "62.0", "67.0")
}
