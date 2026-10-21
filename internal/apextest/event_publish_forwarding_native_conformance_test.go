package apextest

import "testing"

func TestEventPublishForwardingNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "event_publish_forwarding", "EventForwardingProbeTest", "62.0", "63.0", "64.0", "65.0", "66.0", "67.0")
}
