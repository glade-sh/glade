@IsTest private class Road544Response62 {
    @IsTest static void saleResponse() {
        commercepayments.SaleResponse response = new commercepayments.SaleResponse();
        commercepayments.GatewayResponse gateway = response;
        System.assert(gateway === response);
    }
    @IsTest static void reversalResponse() {
        commercepayments.AuthorizationReversalResponse response = new commercepayments.AuthorizationReversalResponse();
        commercepayments.GatewayResponse gateway = response;
        System.assert(gateway === response);
    }
}
