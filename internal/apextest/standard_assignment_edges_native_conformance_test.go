package apextest

import "testing"

func TestStandardAssignmentEdgesNativeConformance(t *testing.T) {
	runStandardEventNativeRows(t, "standard_assignment_edges")
}
