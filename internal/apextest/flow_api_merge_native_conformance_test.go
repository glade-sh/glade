package apextest

import "testing"

// NATIVE API endpoint references in a Flow template and formula:
// testdata/conformance/flow_api_merge/org.tsv lines 2-3, isolated tests.
func TestFlowAPIMergeNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "flow_api_merge", 2, nil)
}
