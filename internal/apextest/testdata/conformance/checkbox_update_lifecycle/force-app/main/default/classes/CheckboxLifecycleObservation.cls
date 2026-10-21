@IsTest private class CheckboxLifecycleObservation {
 static User actor() {
  User user=new User(Alias='life',Email='life@example.invalid',EmailEncodingKey='UTF-8',LastName='Lifecycle',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',Username='life-'+UserInfo.getOrganizationId()+'@example.invalid',ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User'].Id);
  insert user; return user;
 }
 static void grant(Id userId,String name) {
  insert new PermissionSetAssignment(AssigneeId=userId,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name=:name].Id);
 }
 static void observe(String actor) {
  List<String> rows=new List<String>();
  for(String shape:new List<String>{'newOmitted','queryOmitted','queryValue','explicitFalse','explicitTrue'}) {
   LifecycleParcel__c seed=new LifecycleParcel__c(Name='Seed');
   Database.insert(seed,AccessLevel.SYSTEM_MODE);
   LifecycleParcel__c row;
   if(shape=='newOmitted') row=new LifecycleParcel__c(Id=seed.Id,Name='Changed');
   else if(shape=='queryOmitted') row=[SELECT Id,Name FROM LifecycleParcel__c WHERE Id=:seed.Id WITH SYSTEM_MODE];
   else row=[SELECT Id,Name,Enabled__c FROM LifecycleParcel__c WHERE Id=:seed.Id WITH SYSTEM_MODE];
   if(shape=='explicitFalse') row.Enabled__c=false;
   if(shape=='explicitTrue') row.Enabled__c=true;
   Boolean populated=row.getPopulatedFieldsAsMap().containsKey('Enabled__c');
   Boolean updateable=LifecycleParcel__c.Enabled__c.getDescribe().isUpdateable();
   SObjectAccessDecision decision=Security.stripInaccessible(AccessType.UPDATABLE,new List<SObject>{row},false);
   Set<String> removed=decision.getRemovedFields().get('LifecycleParcel__c');
   Boolean removedCheckbox=removed!=null && removed.contains('Enabled__c');
   Database.update(row,AccessLevel.SYSTEM_MODE);
   rows.add(shape+':'+populated+','+updateable+','+removedCheckbox);
  }
  System.assert(false,'LIFECYCLE|'+actor+'|'+String.join(rows,';')+'|END');
 }
 @TestSetup static void setup() { grant(UserInfo.getUserId(),'LifecycleFull'); }
 @IsTest static void runningFull() { observe('runningFull'); }
 @IsTest static void identityFull() {
  System.runAs(new User(Id=UserInfo.getUserId())) {
   User identity=[SELECT Id,ProfileId FROM User WHERE Id=:UserInfo.getUserId()];
   System.runAs(identity) { observe('identityFull'); }
  }
 }
 @IsTest static void standardObject() {
  User user=actor();grant(user.Id,'LifecycleObject');
  System.runAs(user) { observe('standardObject'); }
 }
 @IsTest static void standardFull() {
  User user=actor();grant(user.Id,'LifecycleFull');
  System.runAs(user) { observe('standardFull'); }
 }
}
