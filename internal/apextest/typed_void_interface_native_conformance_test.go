package apextest

import "testing"

func TestTypedVoidInterfaceNativeConformance(t *testing.T) {
	runNativeProjectRoutes(t, "typed_void_interface", "EntryHookProbeTest", []string{"62.0", "67.0"}, 8)
}
