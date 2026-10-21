package apextest

import "testing"

func TestIdStringNoopNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "id_string_noop", "IdStringNoopTest", []string{"62.0", "67.0"}, 7)
}
