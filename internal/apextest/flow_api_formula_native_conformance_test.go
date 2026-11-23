package apextest

import "testing"

// NATIVE nested API endpoint formula: org.tsv lines 2-3,
// isolated tests at API 62.0 and 67.0.
func TestFlowAPIFormulaNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "flow_api_formula", 2, nil)
}
