public without sharing class HierarchyBoundaryProbe {
 public static Map<String,Object> observe(Boolean overrideUser) {
  insert new BoundaryPreference__c(SetupOwnerId=UserInfo.getOrganizationId(),Disabled__c=true);
  Map<String,Object> result=new Map<String,Object>{'adminWith'=>BoundarySharedReader.read(),'adminWithout'=>BoundarySystemReader.read()};
  if(!Test.isRunningTest()) { result.put('boundary','runAsOnlyInTests');return result; }
  User reader=new User(Alias='bound',Email='boundary@example.test',EmailEncodingKey='UTF-8',LastName='Boundary',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id,Username='boundary.'+String.valueOf(Crypto.getRandomLong())+'.'+UserInfo.getOrganizationId()+'@example.test');
  insert reader;
  if(overrideUser) insert new BoundaryPreference__c(SetupOwnerId=reader.Id,Disabled__c=false);
  System.runAs(reader) { result.put('readerWith',BoundarySharedReader.read());result.put('readerWithout',BoundarySystemReader.read()); }
  return result;
 }
 public static Map<String,Object> all() {
  Savepoint point=Database.setSavepoint();Map<String,Object> result=new Map<String,Object>();
  result.put('organization',observe(false));Database.rollback(point);
  result.put('override',observe(true));Database.rollback(point);return result;
 }
}
