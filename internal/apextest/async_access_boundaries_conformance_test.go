package apextest

import "testing"

func TestAsyncAccessBoundariesAnonymousIsTestNamedVMNativeConformance(t *testing.T) {
	runRuntimeShapeFixture(t, "testdata/conformance/async_access_boundaries", "AsyncAccessProbe", "AsyncAccessProbeTest", 12, 0, nil)
}
