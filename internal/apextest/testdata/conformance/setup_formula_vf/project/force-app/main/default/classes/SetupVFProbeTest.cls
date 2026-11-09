@IsTest private class SetupVFProbeTest {
 @IsTest static void actor() { probe(false); }
 @IsTest static void control() { probe(true); }
 private static void probe(Boolean flag) {
  User actor=new User(Alias='sfprobe',Email='synthetic@example.invalid',EmailEncodingKey='UTF-8',LastName='Probe',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=UserInfo.getProfileId(),Username='synthetic.vf.'+String.valueOf(DateTime.now().getTime())+'.'+String.valueOf(flag)+'@example.invalid');insert actor;
  insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='SetupProbe'].Id);
  System.runAs(actor) {
   insert new Prefs__c(Name='Org',SetupOwnerId=UserInfo.getOrganizationId(),Flag__c=true);
   insert new Prefs__c(Name='Actor',SetupOwnerId=UserInfo.getUserId(),Flag__c=flag);
   Account a=new Account(Name='Probe');insert a;Test.setCurrentPage(Page.SetupPage);
   ApexPages.StandardController c=new ApexPages.StandardController(new Account(Id=a.Id));c.addFields(new List<String>{'SetupFlag__c'});
   Account out=(Account)c.getRecord();System.assert(false,'OBS|'+JSON.serialize(new Map<String,Object>{'value'=>out.SetupFlag__c}));
  }
 }
}
