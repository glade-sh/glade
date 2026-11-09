package apextest

import "testing"

func TestComponentTabIdentityCase159NativeConformance(t *testing.T) {
	runNativeProjectRoutes(t, "case159_component_tab_identity", "Case159TabEqualityTest", []string{"62.0", "67.0", "68.0"}, 1)
}
