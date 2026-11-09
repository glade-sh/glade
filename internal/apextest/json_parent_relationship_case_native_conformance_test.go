package apextest

import "testing"

func TestJSONParentRelationshipCaseNativeConformance(t *testing.T) {
	runRuntimeShapeFixture(t, "testdata/conformance/json_parent_relationship_case", "Probe2712", "Probe2712Test", 2, 0, nil)
}
