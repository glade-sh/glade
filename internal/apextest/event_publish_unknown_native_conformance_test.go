package apextest

import "testing"

func TestEventPublishUnknownNativeConformance(t *testing.T) {
	runStandardEventNativeRows(t, "event_publish_unknown")
}
