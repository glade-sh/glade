@IsTest private class OwnedPaymentContextProbe {
 private class ResponseMarker implements commercepayments.GatewayResponse {}
 @IsTest static void responseMarkerControl() {
  commercepayments.GatewayResponse response=new ResponseMarker();
  System.assert(response instanceof ResponseMarker);
 }
 @IsTest static void refundRequestRoundTrip() {
  Double amount=8.0;
  commercepayments.ReferencedRefundRequest refund=new commercepayments.ReferencedRefundRequest(amount,null);
  commercepayments.RefundRequest refundBase=refund;
  commercepayments.PaymentGatewayRequest request=refundBase;
  commercepayments.PaymentGatewayContext context=new commercepayments.PaymentGatewayContext(request,commercepayments.RequestType.ReferencedRefund);
  System.assert(context.getPaymentRequest() === refund);
  System.assertEquals(commercepayments.RequestType.ReferencedRefund,context.getPaymentRequestType());
 }

 @IsTest static void contextRoundTrip() {
  Double amount=12.5;
  commercepayments.AuthorizationRequest request = new commercepayments.AuthorizationRequest(amount);
  request.currencyIsoCode='USD';
  commercepayments.PaymentGatewayContext context = new commercepayments.PaymentGatewayContext(request, commercepayments.RequestType.Authorize);
  System.assertEquals(commercepayments.RequestType.Authorize,context.getPaymentRequestType());
  commercepayments.AuthorizationRequest roundTrip=(commercepayments.AuthorizationRequest)context.getPaymentRequest();
  System.assertEquals(12.5,roundTrip.amount);
  System.assertEquals('USD',roundTrip.currencyIsoCode);
  System.assert(request === roundTrip);
  roundTrip.amount=25.0;
  System.assertEquals(25.0,request.amount);
 }
 @IsTest static void requestBaseRoundTrip() {
  Double amount=2.0;
  commercepayments.CaptureRequest capture = new commercepayments.CaptureRequest(amount,null);
  commercepayments.BaseRequest baseRequest = capture;
  commercepayments.PaymentGatewayRequest gatewayRequest=baseRequest;
  System.assert(gatewayRequest instanceof commercepayments.CaptureRequest);
  commercepayments.PaymentGatewayContext context = new commercepayments.PaymentGatewayContext(gatewayRequest,commercepayments.RequestType.Capture);
  System.assert(context.getPaymentRequest() === capture);
  System.assertEquals(commercepayments.RequestType.Capture,context.getPaymentRequestType());
  commercepayments.AuthorizationRequest auth=new commercepayments.AuthorizationRequest(amount);
  Object authBox=auth;
  System.assert(authBox instanceof commercepayments.BaseRequest);
  System.assert(authBox instanceof commercepayments.PaymentGatewayRequest);
 }
 @IsTest static void responseRoundTrips() {
  commercepayments.GatewayResponse response = new commercepayments.CaptureResponse();
  System.assert(response instanceof commercepayments.CaptureResponse);
  commercepayments.GatewayResponse postResponse = new commercepayments.PostAuthorizationResponse();
  System.assert(postResponse instanceof commercepayments.PostAuthorizationResponse);
  System.assert(!(postResponse instanceof commercepayments.CaptureResponse));
  commercepayments.GatewayResponse authResponse=new commercepayments.AuthorizationResponse();
  commercepayments.GatewayResponse refundResponse=new commercepayments.ReferencedRefundResponse();
  commercepayments.GatewayResponse errorResponse=new commercepayments.GatewayErrorResponse('400','owned');
  System.assert(authResponse instanceof commercepayments.AuthorizationResponse);
  System.assert(refundResponse instanceof commercepayments.ReferencedRefundResponse);
  System.assert(errorResponse instanceof commercepayments.GatewayErrorResponse);
 }
 @IsTest static void notificationRoundTrip() {
  commercepayments.BaseNotification notification = new commercepayments.ReferencedRefundNotification();
  System.assert(notification instanceof commercepayments.ReferencedRefundNotification);
  System.assert(!(notification instanceof commercepayments.CaptureNotification));
 }
 @IsTest static void nullContext() {
  commercepayments.PaymentGatewayContext context=new commercepayments.PaymentGatewayContext(null,null);
  System.assertEquals(null,context.getPaymentRequest());
  System.assertEquals(null,context.getPaymentRequestType());
 }
 @IsTest static void requestControls() {
  Double amount=3.5;
  commercepayments.AuthorizationRequest request=new commercepayments.AuthorizationRequest(amount);
  System.assertEquals(3.5,request.amount);
  request.currencyIsoCode='EUR';
  System.assertEquals('EUR',request.currencyIsoCode);
  Object boxed=request;
  System.assert(!(boxed instanceof commercepayments.GatewayResponse));
  Object response=new commercepayments.CaptureResponse();
  System.assert(!(response instanceof commercepayments.PaymentGatewayRequest));
 }
}
