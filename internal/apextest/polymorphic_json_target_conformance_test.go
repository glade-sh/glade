package apextest

import "testing"

func TestPolymorphicJSONTargetOrgConformance(t *testing.T) {
	runNativeObservationConformance(t, "polymorphic_json_target.json", "PolymorphicJSONTarget", 9)
}
