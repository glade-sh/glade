package apextest

import "testing"

func TestAccessLevelIdentityAndJSONAnonymousIsTestNamedVMNativeConformance(t *testing.T) {
	runRuntimeShapeFixture(t, "testdata/conformance/access_level_json_routes", "AccessJsonProbe", "AccessJsonProbeTest", 8, 0, nil)
}
