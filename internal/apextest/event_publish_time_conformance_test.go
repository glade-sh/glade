package apextest

import "testing"

func TestEventPublishTimeNativeConformance(t *testing.T) {
	runMetadataObservationConformance(t, "event_publish_time.json", 6, 0, 0, 0)
}
