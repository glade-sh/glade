void probe() {
    commercepayments.SaleResponse response = new commercepayments.SaleResponse();
    commercepayments.GatewayResponse gateway = response;
    System.assert(gateway === response);
}
probe();
