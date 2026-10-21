package apextest

import "testing"

func TestHandledMessageBoundaryNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "handled_message_boundary", "HandledMessageProbeTest", "62.0", "67.0")
}
