package apextest

import "testing"

func TestFlowParentBeforeReviewNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "flow_parent_before_review", 6, nil)
}
