@IsTest private class Road544ReversalDecimal62 {
    @IsTest static void probe() {
        commercepayments.AuthorizationReversalRequest request = new commercepayments.AuthorizationReversalRequest(12.25, 'ref');
        commercepayments.PaymentGatewayContext context = new commercepayments.PaymentGatewayContext(request, commercepayments.RequestType.AuthorizationReversal);
        System.assert(context.getPaymentRequest() === request);
    }
}
