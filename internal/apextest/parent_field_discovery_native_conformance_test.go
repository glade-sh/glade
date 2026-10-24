package apextest

import "testing"

func TestParentFieldDiscoveryAnonymousAndIsTestNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "parent_field_discovery", "ParentDiscoveryProbeTest", "62.0", "67.0")
}
