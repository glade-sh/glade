package apextest

import "testing"

func TestIDSetMatchingNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "id_set_matching_native", "IdSetMatchingTest", []string{"62.0", "67.0"}, 5)
}
