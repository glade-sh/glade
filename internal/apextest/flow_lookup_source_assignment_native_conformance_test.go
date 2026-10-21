package apextest

import "testing"

func TestFlowLookupSourceAssignmentNativeConformance(t *testing.T) {
	runFlowRecordAssignmentNativeConformance(t, "flow_lookup_source_assignment", "GroveLookupSourceProbe", "GroveLookupSourceProbeTest", "observeLookupSource", "Grove_Lookup_Source", "FLOW_LOOKUP_SOURCE_ASSIGNMENT", []string{"P01", "P02", "P03"})
}
