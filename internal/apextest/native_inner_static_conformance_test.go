package apextest

import "testing"

func TestNativeInnerStaticOrgConformance(t *testing.T) {
	runNativeBoundaryConformance(t, "native_inner_static.json", "C115W4", 6)
}
