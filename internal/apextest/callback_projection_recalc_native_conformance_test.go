package apextest

import "testing"

func TestCallbackProjectionRecalcNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "callback_projection_recalc", "CallbackProjectionProbeTest", "62.0", "67.0")
}
