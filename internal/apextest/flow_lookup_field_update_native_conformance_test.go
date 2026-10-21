package apextest

import "testing"

func TestFlowLookupFieldUpdateNativeConformance(t *testing.T) {
	runFlowRecordAssignmentNativeConformance(t, "flow_lookup_field_update", "GroveLookupFieldProbe", "GroveLookupFieldProbeTest", "observeLookupField", "Grove_Lookup_Field", "FLOW_LOOKUP_FIELD_UPDATE", []string{"L01", "L02", "L03", "L04"})
}
