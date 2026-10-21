package apextest

import "testing"

func TestAbstractRedeclarationOrgConformance(t *testing.T) {
	runNativeObservationConformance(t, "abstract_redeclaration.json", "AbstractRedeclaration", 6)
}
