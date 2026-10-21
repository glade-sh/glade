@IsTest private class JunctionVisibilityTest {
 @TestSetup static void setup() {
  PrivateBoard__c board=new PrivateBoard__c(Name='Seed'); insert board;
  PublicResource__c resource=new PublicResource__c(Name='Seed'); insert resource;
  insert new AllocationLink__c(Name='Seed',Board__c=board.Id,Resource__c=resource.Id);
 }
 @IsTest static void visible_records() {
  Profile profile=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1];
  User actor=new User(Username='junction.'+UserInfo.getOrganizationId()+'@example.invalid',Alias='junc',Email='junction@example.invalid',LastName='JunctionReader',ProfileId=profile.Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US');insert actor;
  PermissionSet grant=[SELECT Id FROM PermissionSet WHERE Name='JunctionRead' LIMIT 1];insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=grant.Id);
  Map<String,Object> observed=new Map<String,Object>{'outside'=>[SELECT COUNT() FROM AllocationLink__c]};
  System.runAs(actor) {
   observed.put('private',[SELECT COUNT() FROM PrivateBoard__c]);observed.put('public',[SELECT COUNT() FROM PublicResource__c]);observed.put('junction',[SELECT COUNT() FROM AllocationLink__c]);
  }
  System.assert(false,'OBSERVED|visible_records|'+JSON.serialize(observed));
 }
}
