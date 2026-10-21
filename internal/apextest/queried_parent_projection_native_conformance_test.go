package apextest

import "testing"

func TestQueriedParentProjectionNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "queried_parent_projection", "ParentProjectionProbeTest", "62.0", "67.0")
}
