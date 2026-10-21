package apextest

import "testing"

func TestFormulaBlankDateSubtractionNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "formula_blank_date_subtraction_native", "GroveFormulaObservation", []string{"62.0", "67.0"}, 1)
}
