package apextest

import "testing"

func TestPaymentRequestIdentityNativeConformance(t *testing.T) {
	runNativeObservationProject(t, "payment_request_identity", "PaymentIdentityTest", []string{"62.0", "67.0"}, 7)
}
