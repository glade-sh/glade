commercepayments.AuthorizationReversalRequest request = new commercepayments.AuthorizationReversalRequest(12.25, 'ref');
commercepayments.PaymentGatewayContext context = new commercepayments.PaymentGatewayContext(request, commercepayments.RequestType.AuthorizationReversal);
System.debug('ROAD544:' + (context.getPaymentRequest() === request));
