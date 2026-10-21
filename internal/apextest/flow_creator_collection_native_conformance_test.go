package apextest

import "testing"

func TestFlowCreatorCollectionNativeConformance(t *testing.T) {
	runFlowRecordAssignmentNativeConformance(t, "flow_creator_collection", "GroveCreatorProbe", "GroveCreatorProbeTest", "observeCreator", "Grove_Creator", "FLOW_CREATOR_COLLECTION", []string{"C01", "C02", "C03"})
}
