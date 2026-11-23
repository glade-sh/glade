@IsTest private class ProbeRoutes {
 @IsTest static void query() {
  String q = 'SELECT Id, (SELECT Id FROM Contacts WHERE Email != null), (SELECT Id, Name FROM Opportunities WHERE StageName = \'Prospecting\' AND Id IN (SELECT OpportunityId FROM OpportunityContactRole WHERE Role = \'Decision Maker\') LIMIT 1), (SELECT Id FROM Cases LIMIT 1) FROM Account';
  Database.query(q);
 }
 @IsTest static void metadata() {
  ProbeFlag__mdt value = ProbeFlag__mdt.getInstance('Case');
  System.assertNotEquals(null, value); System.assertEquals(true, value.Active__c);
  System.assertEquals(null, ProbeFlag__mdt.getInstance('Missing'));
 }
}
