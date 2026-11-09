@IsTest private class ProbeUsers {
 static User makeUser(String a) {
  Profile p=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  User u=new User(ProfileId=p.Id,Alias=a,LastName='Probe',Username=a+Crypto.getRandomLong()+'@example.invalid',Email='probe@example.invalid',TimeZoneSidKey='GMT',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US'); insert u; return u;
 }
 static void probe(Boolean grant) {
  User actor=makeUser('actor'); User target=makeUser('target');
  if(grant) insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='ProbeGrant'].Id);
  String result='';
  System.runAs(actor) { try { update new User(Id=target.Id,LastName='Changed'); result='ok'; } catch(DmlException e) {result=String.valueOf(e.getDmlType(0));} }
  String stored=[SELECT LastName FROM User WHERE Id=:target.Id].LastName;
  System.assert(false,'CAP|'+result+'|'+stored);
 }
 @IsTest static void privileged() {probe(true);}
 @IsTest static void unprivileged() {probe(false);}
}
