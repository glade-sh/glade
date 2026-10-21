package apextest

import "testing"

func TestWorkOrderChildCountNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "work_order_child_count", "MeadowWorkCountProbeTest", "62.0", "67.0")
}
