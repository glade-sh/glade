package apextest

import "testing"

func TestSourceEnumAccessLevelJSONAnonymousIsTestNamedVMNativeConformance(t *testing.T) {
	runRuntimeShapeFixture(t, "testdata/conformance/source_enum_access_json_routes", "SourceEnumJsonProbe", "SourceEnumJsonProbeTest", 4, 0, nil)
}
