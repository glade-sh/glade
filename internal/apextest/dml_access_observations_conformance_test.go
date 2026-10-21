package apextest

import "testing"

func TestDMLAccessMessageNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "dml_access_observations", "RecordAccessOperationProbeTest", []string{"62.0", "67.0", "68.0"}, 5)
}
