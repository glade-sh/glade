@IsTest private class ContextAdapterTest {
 @IsTest static void dispatch() {
  commercepayments.PaymentGatewayAdapter adapter = new ContextAdapter();
  Double amount = 9.0;
  commercepayments.PaymentGatewayContext context = new commercepayments.PaymentGatewayContext(new commercepayments.AuthorizationRequest(amount), commercepayments.RequestType.Authorize);
  System.assert(adapter.processRequest(context) instanceof commercepayments.CaptureResponse);
 }
}
