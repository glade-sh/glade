@IsTest private class R6PermissionTest {
 static User makeUser() {
  String uniqueName = 'r6.' + String.valueOf(Crypto.getRandomInteger()).replace('-','n') + '@example.invalid';
  User u = new User(Username=uniqueName, Email=uniqueName, Alias='r6test', LastName='Probe', TimeZoneSidKey='America/Los_Angeles', LocaleSidKey='en_US', EmailEncodingKey='UTF-8', LanguageLocaleKey='en_US', ProfileId=UserInfo.getProfileId());
  insert u; return u;
 }
 @IsTest static void noPermission() {
  User u = makeUser(); System.runAs(u) { System.assertEquals(false, FeatureManagement.checkPermission('R6_Bypass')); System.assertEquals('PASS', R6Probe.observe('R6Perm')); }
 }
 @IsTest static void withPermission() {
  User u = makeUser(); PermissionSet ps = [SELECT Id FROM PermissionSet WHERE Name='R6_Bypass' LIMIT 1];
  insert new PermissionSetAssignment(AssigneeId=u.Id, PermissionSetId=ps.Id);
  System.runAs(u) { System.assertEquals(true, FeatureManagement.checkPermission('R6_Bypass')); System.assertEquals('MATCH', R6Probe.observe('R6Perm')); }
 }
}
