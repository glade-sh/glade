package apextest

import "testing"

func TestQualifiedInterfaceOverloadNativeConformance(t *testing.T) {
	runQueryCompositionNativeConformance(t, "qualified_interface_overload", "PlanProbeTest", 3)
}
