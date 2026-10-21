package apextest

import "testing"

func TestLineItemProductNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "line_item_product", "LineItemProductObservationTest", []string{"62.0", "67.0"}, 1)
}
