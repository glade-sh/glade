package apextest

import "testing"

func TestAggregateFieldCapabilitiesNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "aggregate_field_capabilities", "AggregateCapabilitiesProbeTest", "62.0", "67.0")
}
