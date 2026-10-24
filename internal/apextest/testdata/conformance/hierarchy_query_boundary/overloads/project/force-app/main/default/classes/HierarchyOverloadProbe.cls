public without sharing class HierarchyOverloadProbe {
 public static Map<String,Object> observe(Boolean grantAccess) {
  if(Test.isRunningTest()) {
   System.runAs(new User(Id=UserInfo.getUserId())) { insert new BoundaryPreference__c(SetupOwnerId=UserInfo.getOrganizationId(),Disabled__c=true); }
  } else { insert new BoundaryPreference__c(SetupOwnerId=UserInfo.getOrganizationId(),Disabled__c=true); }
  Map<String,Object> result=new Map<String,Object>{'adminWith'=>OverloadSharedReader.read(),'adminWithout'=>OverloadSystemReader.read()};
  if(!Test.isRunningTest()) { result.put('boundary','runAsOnlyInTests');return result; }
  User reader=new User(Alias='overld',Email='overload@example.test',EmailEncodingKey='UTF-8',LastName='Overload',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id,Username='overload.'+String.valueOf(Crypto.getRandomLong())+'.'+UserInfo.getOrganizationId()+'@example.test');
  insert reader;
  if(grantAccess) insert new PermissionSetAssignment(AssigneeId=reader.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='BoundarySettingsReader'].Id);
  System.runAs(reader) { result.put('readerWith',OverloadSharedReader.read());result.put('readerWithout',OverloadSystemReader.read()); }
  return result;
 }
}
