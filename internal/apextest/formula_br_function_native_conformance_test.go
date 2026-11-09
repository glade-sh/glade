package apextest

import "testing"

// BR() returns "<br>" (no self-closing slash) in a formula text field. Native
// rows at API 62.0 and 67.0, anonymous and isTest.
func TestFormulaBrFunctionNativeConformance(t *testing.T) {
	runInnerTypeNativeConformanceWithComparison(t, "formula_br_function", "FormulaBrProbeTest", nil, "62.0", "67.0")
}
