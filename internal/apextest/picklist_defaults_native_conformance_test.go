package apextest

import "testing"

func TestPicklistDefaultsNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "picklist_defaults", "PicklistDefaultsObservationTest", []string{"62.0", "67.0"}, 2)
}
