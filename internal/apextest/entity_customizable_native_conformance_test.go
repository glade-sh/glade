package apextest

import "testing"

func TestEntityCustomizableNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "entity_customizable", "EntityCapabilityTest", []string{"62.0", "67.0"}, 2)
}
