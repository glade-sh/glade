package apextest

import "testing"

func TestDynamicSOQLClosedScopeNativeConformance(t *testing.T) {
	runScopedNativeConformance(t, "dynamic_bind_closed_scope_native.json", "ClosedScopeBindObservation", 10, 60, 0)
}
