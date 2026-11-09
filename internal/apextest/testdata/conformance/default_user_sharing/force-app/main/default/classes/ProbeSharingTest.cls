@IsTest private class ProbeSharingTest {
 @IsTest static void observe() {
  Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  User actor=new User(ProfileId=p.Id,Alias='actor',LastName='Probe',Username='share'+Crypto.getRandomLong()+'@example.invalid',Email='probe@example.invalid',TimeZoneSidKey='GMT',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US'); insert actor; insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='ProbeObject'].Id);
  ProbeItem__c item; System.runAs(new User(Id=UserInfo.getUserId())) {item=new ProbeItem__c(Name='Original'); insert item;}
  String admin; System.runAs(new User(Id=UserInfo.getUserId())) {admin=ProbeSharing.observe(item.Id);} String standard; System.runAs(actor) {standard=ProbeSharing.observe(item.Id);}
  System.assert(false,'CAP|'+admin+'|'+standard);
 }
}
