package apextest

import "testing"

func TestCustomAccessLevelFeatureGateNativeConformance(t *testing.T) {
	runNamedObservationNativeConformance(t, "custom_access_level_feature_gate", "AccessLevelPermSetFeatureProbe", "67.0")
}
