package apextest

import "testing"

func TestQueryLikeNegationNativeConformance(t *testing.T) {
	runQueryCompositionNativeConformance(t, "query_like_negation", "QueryOperatorProbeTest", 4)
}
