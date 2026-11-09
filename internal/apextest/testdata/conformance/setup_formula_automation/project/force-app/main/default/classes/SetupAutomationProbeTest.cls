@IsTest private class SetupAutomationProbeTest {
 @IsTest static void actor() {
  User actor=new User(Alias='saprobe', Email='synthetic@example.invalid', EmailEncodingKey='UTF-8', LastName='Probe', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles', ProfileId=UserInfo.getProfileId(), Username='synthetic.automation.'+String.valueOf(DateTime.now().getTime())+'@example.invalid');
  insert actor;
  insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='SetupProbe'].Id);
  System.runAs(actor) {
   insert new Prefs__c(Name='Org',SetupOwnerId=UserInfo.getOrganizationId(),Flag__c=true);
   insert new Prefs__c(Name='Actor',SetupOwnerId=UserInfo.getUserId(),Flag__c=false);
   Account a=new Account(Name='Probe');insert a;
   Account observed=[SELECT FlowResult__c, WFResult__c FROM Account WHERE Id=:a.Id];
   System.assert(false,'OBS|'+JSON.serialize(new Map<String,Boolean>{'flow'=>observed.FlowResult__c,'workflow'=>observed.WFResult__c}));
  }
 }
}
