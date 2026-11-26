commercepayments.SaleRequest request = new commercepayments.SaleRequest(12.25);
commercepayments.PaymentGatewayContext context = new commercepayments.PaymentGatewayContext(request, commercepayments.RequestType.Sale);
System.debug('ROAD544:' + (context.getPaymentRequest() === request));
