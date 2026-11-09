@IsTest private class SetupFlowTypesProbeTest {
 @IsTest static void actor() { probe(10,'Probe'); }
 @IsTest static void control() { probe(1,'other'); }
 private static void probe(Integer amount,String phrase) {
  User actor=new User(Alias='stprobe',Email='synthetic@example.invalid',EmailEncodingKey='UTF-8',LastName='Probe',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=UserInfo.getProfileId(),Username='synthetic.types.'+String.valueOf(DateTime.now().getTime())+'.'+String.valueOf(amount)+'@example.invalid');insert actor;
  insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='SetupProbe'].Id);
  System.runAs(actor) {
   insert new Prefs__c(Name='Actor',SetupOwnerId=UserInfo.getUserId(),Amount__c=amount,Phrase__c=phrase);
   Account a=new Account(Name='Probe');insert a;
   Account observed=[SELECT NumberMatch__c,TextMatch__c FROM Account WHERE Id=:a.Id];
   System.assert(false,'OBS|'+JSON.serialize(new Map<String,Boolean>{'numeric'=>observed.NumberMatch__c,'text'=>observed.TextMatch__c}));
  }
 }
}
