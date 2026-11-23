global class ContextAdapter implements commercepayments.PaymentGatewayAdapter {
 global commercepayments.GatewayResponse processRequest(commercepayments.PaymentGatewayContext context) {
  System.assertEquals(commercepayments.RequestType.Authorize, context.getPaymentRequestType());
  commercepayments.AuthorizationRequest request = (commercepayments.AuthorizationRequest) context.getPaymentRequest();
  System.assertEquals(9.0, request.amount);
  return new commercepayments.CaptureResponse();
 }
}
