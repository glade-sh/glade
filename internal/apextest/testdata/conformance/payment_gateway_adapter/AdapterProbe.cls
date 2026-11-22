global class AdapterProbe implements commercepayments.PaymentGatewayAdapter {
 global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) {
  System.assertEquals(null, context);
  return new commercepayments.CaptureResponse();
 }
}
