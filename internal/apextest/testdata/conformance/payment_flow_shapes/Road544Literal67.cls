@IsTest private class Road544Literal67 {
    @IsTest static void saleInteger() {
        commercepayments.SaleRequest request = new commercepayments.SaleRequest(12);
        commercepayments.PaymentGatewayContext context = new commercepayments.PaymentGatewayContext(request, commercepayments.RequestType.Sale);
        System.assert(context.getPaymentRequest() === request);
    }
    @IsTest static void saleDouble() {
        commercepayments.SaleRequest request = new commercepayments.SaleRequest((Double)12.25);
        commercepayments.PaymentGatewayContext context = new commercepayments.PaymentGatewayContext(request, commercepayments.RequestType.Sale);
        System.assert(context.getPaymentRequest() === request);
    }
    @IsTest static void reversalInteger() {
        commercepayments.AuthorizationReversalRequest request = new commercepayments.AuthorizationReversalRequest(12, 'ref');
        commercepayments.PaymentGatewayContext context = new commercepayments.PaymentGatewayContext(request, commercepayments.RequestType.AuthorizationReversal);
        System.assert(context.getPaymentRequest() === request);
    }
    @IsTest static void reversalDouble() {
        commercepayments.AuthorizationReversalRequest request = new commercepayments.AuthorizationReversalRequest((Double)12.25, 'ref');
        commercepayments.PaymentGatewayContext context = new commercepayments.PaymentGatewayContext(request, commercepayments.RequestType.AuthorizationReversal);
        System.assert(context.getPaymentRequest() === request);
    }
}
