package apextest

import "testing"

func TestInheritedFinalAssignmentOrgConformance(t *testing.T) {
	runNativeObservationConformance(t, "inherited_final_assignment.json", "InheritedFinalAssignment", 6)
}
