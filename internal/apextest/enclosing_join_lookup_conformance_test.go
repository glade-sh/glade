package apextest

import "testing"

func TestEnclosingJoinLookupOrgConformance(t *testing.T) {
	runNativeBoundaryConformance(t, "enclosing_join_lookup.json", "C125C09", 6)
}
