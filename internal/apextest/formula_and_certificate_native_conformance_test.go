package apextest

import "testing"

func TestFormulaAndCertificateNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "formula_and_certificate", "TailSurfaceTest", []string{"62.0", "67.0"}, 2)
}
