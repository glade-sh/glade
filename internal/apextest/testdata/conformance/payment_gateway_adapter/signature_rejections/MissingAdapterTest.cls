@IsTest private class MissingAdapterTest implements commercepayments.PaymentGatewayAdapter {
 @IsTest static void verify() { System.assertEquals(1, AdapterSupport.value()); }
}
