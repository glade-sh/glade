package apextest

import "testing"

func TestQueryProjectionBoundaryNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "query_projection_boundary", "QueryProjectionProbeTest", "62.0", "67.0")
}
