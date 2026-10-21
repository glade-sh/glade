package apextest

import "testing"

func TestInheritedCurrencyNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "inherited_currency", "DeliveryCurrencyTest", []string{"62.0", "67.0"}, 2)
}
