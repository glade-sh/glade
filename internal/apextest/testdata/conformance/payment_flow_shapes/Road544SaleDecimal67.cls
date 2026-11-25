@IsTest private class Road544SaleDecimal67 {
    @IsTest static void probe() {
        commercepayments.SaleRequest request = new commercepayments.SaleRequest(12.25);
        commercepayments.PaymentGatewayContext context = new commercepayments.PaymentGatewayContext(request, commercepayments.RequestType.Sale);
        System.assert(context.getPaymentRequest() === request);
    }
}
