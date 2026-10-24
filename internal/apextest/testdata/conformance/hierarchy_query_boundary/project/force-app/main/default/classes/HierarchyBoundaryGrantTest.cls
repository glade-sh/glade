@IsTest private class HierarchyBoundaryGrantTest {
 @IsTest static void observe() {
  System.runAs(new User(Id=UserInfo.getUserId())) { insert new BoundaryPreference__c(SetupOwnerId=UserInfo.getOrganizationId(),Disabled__c=true); }
  User reader=new User(Alias='grant',Email='grant@example.test',EmailEncodingKey='UTF-8',LastName='Grant',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User' LIMIT 1].Id,Username='grant.'+String.valueOf(Crypto.getRandomLong())+'.'+UserInfo.getOrganizationId()+'@example.test');
  insert reader;
  insert new PermissionSetAssignment(AssigneeId=reader.Id,PermissionSetId=[SELECT Id FROM PermissionSet WHERE Name='BoundarySettingsReader'].Id);
  Map<String,Object> result=new Map<String,Object>();
  System.runAs(reader) { result.put('readerWith',BoundarySharedReader.read());result.put('readerWithout',BoundarySystemReader.read()); }
  System.assert(false,'OBSERVED|'+JSON.serialize(new Map<String,Object>{'organization'=>result,'override'=>result}));
 }
}
