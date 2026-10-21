package apextest

import "testing"

func TestNativeTransportControlOrgConformance(t *testing.T) {
	runNativeObservationConformance(t, "native_transport_control.json", "NativeTransportControl", 3)
}
