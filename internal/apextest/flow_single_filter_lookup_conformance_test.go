package apextest

import "testing"

func TestFlowSingleFilterLookupNativeConformance(t *testing.T) {
	runSyntheticObservationNativeConformance(t, "flow_single_filter_lookup", "SingleFilterProbe", "observeLookup", "SINGLE_FILTER_RESULT", 3, false)
}
