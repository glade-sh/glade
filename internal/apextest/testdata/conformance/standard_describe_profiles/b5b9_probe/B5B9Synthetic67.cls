@IsTest private class B5B9Synthetic67 {
 @IsTest static void probe() {
  System.QueueableDuplicateSignature.Builder b = QueueableDuplicateSignature.builder();
  System.assertEquals(0, b.getSize());
  System.assertNotEquals(null, b.addString('job').addInteger(42).addId('001000000000001AAA').build());
  Map<String,Schema.SObjectType> m=Schema.getGlobalDescribe();
  System.assert(m.containsKey('PlatformCachePartition'));
  System.assert(m.get('PricebookEntry').getDescribe().fields.getMap().containsKey('ProductSellingModelId'));
  System.assert(m.containsKey('BuyerGroup'));
  System.assertEquals(false,m.get('BuyerGroup').getDescribe().fields.getMap().containsKey('StrikethroughPricebookId'));
  for (String f : new List<String>{'Identifier','ExemptionId','EntityUseCode'}) System.assertEquals(false,m.get('Customer').getDescribe().fields.getMap().containsKey(f));
 }
}
