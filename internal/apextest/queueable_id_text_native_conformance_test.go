package apextest

import "testing"

func TestQueueableIdTextNativeConformanceIsTestProjectAPIRoutes(t *testing.T) {
	runNativeObservationProject(t, "queueable_id_text", "QueueableIdTextProbeTest", []string{"62.0", "67.0"}, 2)
}
