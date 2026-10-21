package apextest

import "testing"

func TestEventPublishDefaultsOrgConformance(t *testing.T) {
	runMetadataObservationConformance(t, "event_publish_defaults.json", 45, 6, 0, 9)
}
