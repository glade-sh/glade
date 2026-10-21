package apextest

import "testing"

func TestQueueCapacityNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "queue_capacity", "QueueCapacityObservationTest", []string{"62.0", "67.0"}, 2)
}
