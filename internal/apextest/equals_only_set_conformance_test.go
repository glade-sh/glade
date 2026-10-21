package apextest

import "testing"

func TestEqualsOnlySetOrgConformance(t *testing.T) {
	runNativeBoundaryConformance(t, "equals_only_set.json", "C125A2", 8)
}
