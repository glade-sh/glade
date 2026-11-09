@IsTest private class Q1NativeRoutes67 {
 @IsTest static void capture() {
  Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  User u=new User(Alias='q1usr',Email='q1usr@example.invalid',EmailEncodingKey='UTF-8',LastName='Probe',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=p.Id,Username='q1usr'+DateTime.now().getTime()+'@example.invalid');insert u;
  PermissionSet r=[SELECT Id FROM PermissionSet WHERE Name='Probe_Read' LIMIT 1];insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=r.Id);
  String beforeRow;String afterRow;
  System.runAs(u) { beforeRow=observe('no'); }
  PermissionSet g=[SELECT Id FROM PermissionSet WHERE Name='Probe_Bypass_Access' LIMIT 1];insert new PermissionSetAssignment(AssigneeId=u.Id,PermissionSetId=g.Id);
  System.runAs(u) { afterRow=observe('yes'); }
  System.assert(false,'Q1ROUTES|before='+beforeRow+';after='+afterRow);
 }
 static String observe(String expected) {
  Account a=new Account(Name='q1');insert a;Account b=[SELECT Probe_Principal__c,Probe_Granted__c FROM Account WHERE Id=:a.Id];
  String expectedId=String.valueOf(UserInfo.getUserId()).substring(0,15);
  return 'permission='+FeatureManagement.checkPermission('Probe_Bypass')+',principal='+b.Probe_Principal__c.startsWith(expectedId)+',profile='+b.Probe_Principal__c.contains(':Standard User:' + expected)+',workflow='+b.Probe_Granted__c;
 }
}
