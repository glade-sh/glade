package apextest

import "testing"

// NATIVE nested API formula with appended path: org.tsv lines 2-3,
// isolated tests at API 62.0 and 67.0.
func TestFlowAPIFormulaPathNativeConformance(t *testing.T) {
	runFlowCriteriaFixture(t, "flow_api_formula_path", 2, nil)
}
