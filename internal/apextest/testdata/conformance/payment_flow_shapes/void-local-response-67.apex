void probe() {
    commercepayments.AuthorizationReversalResponse response = new commercepayments.AuthorizationReversalResponse();
    commercepayments.GatewayResponse gateway = response;
    System.assert(gateway === response);
}
probe();
