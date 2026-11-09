@IsTest private class SetupOrderingProbeTest {
 @IsTest static void actor() {
  User actor=new User(Alias='soprobe', Email='synthetic@example.invalid', EmailEncodingKey='UTF-8', LastName='Probe', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles', ProfileId=UserInfo.getProfileId(), Username='synthetic.order.'+String.valueOf(DateTime.now().getTime())+'@example.invalid');
  insert actor;
  insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='SetupProbe'].Id);
  System.runAs(actor) {
   insert new Prefs__c(Name='Org',SetupOwnerId=UserInfo.getOrganizationId(),Flag__c=true);
   insert new Prefs__c(Name='Actor',SetupOwnerId=UserInfo.getUserId(),Flag__c=false);
   Account a=new Account(Name='A',Other__c='Z'); Account b=new Account(Name='Z',Other__c='A');insert new List<Account>{a,b};
   insert new List<Contact>{new Contact(LastName='A',Other__c='Z',AccountId=a.Id),new Contact(LastName='Z',Other__c='A',AccountId=a.Id)};
   List<Account> top=[SELECT Name FROM Account ORDER BY Sort__c];
   Account parent=[SELECT (SELECT LastName FROM Contacts ORDER BY Sort__c) FROM Account WHERE Id=:a.Id];
   System.assert(false,'OBS|'+JSON.serialize(new Map<String,String>{'top'=>top[0].Name,'child'=>parent.Contacts[0].LastName}));
  }
 } @IsTest static void control() {
  User actor=new User(Alias='soprobe', Email='synthetic@example.invalid', EmailEncodingKey='UTF-8', LastName='Probe', LanguageLocaleKey='en_US', LocaleSidKey='en_US', TimeZoneSidKey='America/Los_Angeles', ProfileId=UserInfo.getProfileId(), Username='synthetic.order.control.'+String.valueOf(DateTime.now().getTime())+'@example.invalid');
  insert actor;
  insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='SetupProbe'].Id);
  System.runAs(actor) {
   insert new Prefs__c(Name='Org',SetupOwnerId=UserInfo.getOrganizationId(),Flag__c=true);
   insert new Prefs__c(Name='Actor',SetupOwnerId=UserInfo.getUserId(),Flag__c=true);
   Account a=new Account(Name='A',Other__c='Z'); Account b=new Account(Name='Z',Other__c='A');insert new List<Account>{a,b};
   insert new List<Contact>{new Contact(LastName='A',Other__c='Z',AccountId=a.Id),new Contact(LastName='Z',Other__c='A',AccountId=a.Id)};
   List<Account> top=[SELECT Name FROM Account ORDER BY Sort__c];
   Account parent=[SELECT (SELECT LastName FROM Contacts ORDER BY Sort__c) FROM Account WHERE Id=:a.Id];
   System.assert(false,'OBS|'+JSON.serialize(new Map<String,String>{'top'=>top[0].Name,'child'=>parent.Contacts[0].LastName}));
  }
 }
}
