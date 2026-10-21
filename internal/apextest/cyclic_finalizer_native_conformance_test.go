package apextest

import "testing"

func TestCyclicFinalizerNativeConformance(t *testing.T) {
	runNamedObservationNativeConformance(t, "cyclic_finalizer", "CycleFinalizerProbeTest", "62.0", "67.0")
}
