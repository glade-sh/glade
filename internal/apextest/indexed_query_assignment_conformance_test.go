package apextest

import "testing"

func TestIndexedQueryAssignmentOrgConformance(t *testing.T) {
	runNativeBoundaryConformance(t, "indexed_query_assignment.json", "C125C08", 9)
}
