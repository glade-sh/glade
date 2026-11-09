package apextest

import "testing"

func TestJSONParentRelationshipCaseModesNativeConformance(t *testing.T) {
	runRuntimeShapeFixture(t, "testdata/conformance/json_parent_relationship_case_modes", "ProbeMore2712", "ProbeMore2712Test", 2, 0, nil)
}
