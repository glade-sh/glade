@IsTest private class ProbeTest62 {
 @IsTest static void capture() {
  ProbeList__c s = ProbeList__c.getValues('absent');
  System.assertEquals(null,s);
  Boolean caught = false;
  try { String v = s.Text__c; } catch (NullPointerException e) { caught = true; System.assertEquals('Attempt to de-reference a null object', e.getMessage()); }
  System.assert(caught);
  System.assertNotEquals(null,ProbeHierarchy__c.getInstance());
  System.assertNotEquals(null,ProbeHierarchy__c.getOrgDefaults());
  System.assertEquals(1,[SELECT count() FROM NamedCredential WHERE DeveloperName='ProbeEndpoint']);
  System.assertEquals(1,[SELECT count() FROM PaymentGatewayProvider WHERE DeveloperName='ProbeGateway']);
  System.assertEquals(0,[SELECT count() FROM PaymentGatewayProvider WHERE DeveloperName='ProbeMissing']);
 }
}