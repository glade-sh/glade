@IsTest private class SetupRunAsProbeTest {
 @IsTest static void actor() {
  User actor=new User(Alias='sgprobe', Email='synthetic@example.invalid', EmailEncodingKey='UTF-8', LastName='Probe', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles', ProfileId=UserInfo.getProfileId(), Username='synthetic.'+String.valueOf(DateTime.now().getTime())+'@example.invalid');
  insert actor;
  insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='SetupProbe'].Id);
  System.runAs(actor) {
   insert new Prefs__c(Name='Org',SetupOwnerId=UserInfo.getOrganizationId(),Flag__c=true);
   insert new Prefs__c(Name='Actor',SetupOwnerId=UserInfo.getUserId(),Flag__c=false);
   Account a=new Account(Name='Probe');insert a;
   System.assertEquals(false,[SELECT SetupFlag__c FROM Account WHERE Id=:a.Id].SetupFlag__c);
  }
 }
}
