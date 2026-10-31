package apextest

import "testing"

func TestNoRowDynamicControlsNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "no_row_seeds/dynamic", "NoRowDynamicControlTest", []string{"62.0", "67.0"}, 2)
}

func TestNoRowDynamicControlsCachedRoutes(t *testing.T) {
	runNoRowPricebookCachedRoutes(t, "dynamic", "NoRowDynamicControlTest")
}
