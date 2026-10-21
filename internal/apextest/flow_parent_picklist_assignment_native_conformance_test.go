package apextest

import "testing"

func TestFlowParentPicklistAssignmentNativeConformance(t *testing.T) {
	runFlowRecordAssignmentNativeConformance(t, "flow_parent_picklist_assignment", "GroveParentPicklistProbe", "GroveParentPicklistProbeTest", "observeParentPicklist", "Grove_Parent_Picklist", "FLOW_PARENT_PICKLIST_ASSIGNMENT", []string{"K01", "K02"})
}
