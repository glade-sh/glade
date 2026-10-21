package apextest

import "testing"

func TestQueueSetupBoundaryNativeConformance(t *testing.T) {
	runInnerTypeNativeConformance(t, "queue_setup_boundary", "QueueSetupBoundaryProbeTest", "62.0", "67.0")
}

func TestQueueDataBoundaryNativeConformance(t *testing.T) {
	// Anonymous mixed-DML enforcement is a separate runtime boundary. These
	// rows cover the test transaction used for setup and ordinary record work.
	runInnerTypeNativeRoutes(t, "queue_data_boundary", "QueueDataBoundaryProbeTest", []string{"isTest"}, nil, "62.0", "67.0")
}
