package apextest

import "testing"

func TestQueueOwnerShapeNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "queue_owner_shape", "OwnerShapeProbeTest", "62.0", "67.0")
}
