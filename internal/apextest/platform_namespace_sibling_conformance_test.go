package apextest

import "testing"

func TestPlatformNamespaceSiblingNativeConformance(t *testing.T) {
	runSyntheticObservationNativeConformance(t, "platform_namespace_sibling", "NamespaceSiblingProbe", "observeSibling", "NAMESPACE_SIBLING_RESULT", 1, false)
}
