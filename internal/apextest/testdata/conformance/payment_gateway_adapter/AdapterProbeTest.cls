@IsTest private class AdapterProbeTest {
 @IsTest static void dispatch() {
  commercepayments.PaymentGatewayAdapter adapter = new AdapterProbe();
  commercepayments.GatewayResponse result = adapter.processRequest(null);
  System.assert(result instanceof commercepayments.CaptureResponse);
 }
}
