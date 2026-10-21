package apextest

import "testing"

func TestRecordRelationshipWriteNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "record_relationship_write", "RelationshipWriteProbeTest", "62.0", "67.0")
}
