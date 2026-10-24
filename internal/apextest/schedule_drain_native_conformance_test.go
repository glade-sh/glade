package apextest

import "testing"

func TestScheduleDrainDirectBatchFinishQueueableAndScheduledCallbackNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "schedule_drain", "ScheduleDrainObservationTest", []string{"62.0", "67.0"}, 8)
}
