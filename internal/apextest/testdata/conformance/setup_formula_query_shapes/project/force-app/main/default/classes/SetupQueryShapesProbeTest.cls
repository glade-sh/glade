@IsTest private class SetupQueryShapesProbeTest {
 @IsTest static void actor() {
  User actor=new User(Alias='sqprobe',Email='synthetic@example.invalid',EmailEncodingKey='UTF-8',LastName='Probe',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=UserInfo.getProfileId(),Username='synthetic.query.'+String.valueOf(DateTime.now().getTime())+'@example.invalid');insert actor;
  insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='SetupProbe'].Id);
  System.runAs(actor) {
   insert new Prefs__c(Name='Org',SetupOwnerId=UserInfo.getOrganizationId(),Flag__c=true);
   insert new Prefs__c(Name='Actor',SetupOwnerId=UserInfo.getUserId(),Flag__c=false);
   Account a=new Account(Name='A');Account b=new Account(Name='B');insert new List<Account>{a,b};
   insert new Contact(LastName='C',AccountId=a.Id);
   Decimal total=(Decimal)[SELECT SUM(Score__c) total FROM Account][0].get('total');
   Integer joined=[SELECT COUNT() FROM Account WHERE Id IN (SELECT AccountId FROM Contact WHERE Computed__c=false)];
   Contact c=[SELECT ParentFlag__c FROM Contact LIMIT 1];
   Account observed=[SELECT Criterion__c FROM Account WHERE Id=:a.Id];
   System.assert(false,'OBS|'+JSON.serialize(new Map<String,Object>{'aggregate'=>total,'semijoin'=>joined,'nested'=>c.ParentFlag__c,'flowCriterion'=>observed.Criterion__c}));
  }
 } @IsTest static void control() {
  User actor=new User(Alias='sqprobe',Email='synthetic@example.invalid',EmailEncodingKey='UTF-8',LastName='Probe',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=UserInfo.getProfileId(),Username='synthetic.query.control.'+String.valueOf(DateTime.now().getTime())+'@example.invalid');insert actor;
  insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='SetupProbe'].Id);
  System.runAs(actor) {
   insert new Prefs__c(Name='Org',SetupOwnerId=UserInfo.getOrganizationId(),Flag__c=true);
   insert new Prefs__c(Name='Actor',SetupOwnerId=UserInfo.getUserId(),Flag__c=true);
   Account a=new Account(Name='A');Account b=new Account(Name='B');insert new List<Account>{a,b};
   insert new Contact(LastName='C',AccountId=a.Id);
   Decimal total=(Decimal)[SELECT SUM(Score__c) total FROM Account][0].get('total');
   Integer joined=[SELECT COUNT() FROM Account WHERE Id IN (SELECT AccountId FROM Contact WHERE Computed__c=false)];
   Contact c=[SELECT ParentFlag__c FROM Contact LIMIT 1];
   Account observed=[SELECT Criterion__c FROM Account WHERE Id=:a.Id];
   System.assert(false,'OBS|'+JSON.serialize(new Map<String,Object>{'aggregate'=>total,'semijoin'=>joined,'nested'=>c.ParentFlag__c,'flowCriterion'=>observed.Criterion__c}));
  }
 }
}
