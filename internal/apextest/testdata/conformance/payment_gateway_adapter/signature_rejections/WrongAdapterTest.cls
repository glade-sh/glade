@IsTest private class WrongAdapterTest implements commercepayments.PaymentGatewayAdapter {
 public commercepayments.GatewayResponse processRequest(String context) { return null; }
 @IsTest static void verify() { System.assertEquals(1, AdapterSupport.value()); }
}
