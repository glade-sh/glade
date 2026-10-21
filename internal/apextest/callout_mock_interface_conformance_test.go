package apextest

import "testing"

func TestCalloutMockInterfaceOrgConformance(t *testing.T) {
	runNativeObservationConformance(t, "callout_mock_interface.json", "CalloutMockInterface", 7)
}
