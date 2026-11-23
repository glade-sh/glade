@IsTest private class AdapterRejectTest {
 @IsTest static void reject() {
  System.assertEquals(1, AdapterSupport.value());
  commercepayments.PaymentGatewayAdapter adapter = new commercepayments.PaymentGatewayAdapter();
 }
}
