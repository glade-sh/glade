@IsTest private class DefaultAccessProbeTest {
 static User actor() {
  User person=new User(Alias='access',Email='access@example.invalid',EmailEncodingKey='UTF-8',LastName='Access Observer',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',Username='access-'+UserInfo.getOrganizationId()+'@example.invalid',ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User'].Id);
  insert person;
  return person;
 }
 static String failure(Exception error) { return error.getTypeName(); }
 @IsTest static void describeAccess() {
  User person=actor();
  List<Boolean> row=new List<Boolean>();
  System.runAs(person) {
   Schema.DescribeSObjectResult objectInfo=AccessNote__c.SObjectType.getDescribe();
   Schema.DescribeFieldResult fieldInfo=AccessNote__c.Detail__c.getDescribe();
   row.add(objectInfo.isAccessible());row.add(objectInfo.isCreateable());row.add(objectInfo.isUpdateable());row.add(objectInfo.isDeletable());
   row.add(fieldInfo.isAccessible());row.add(fieldInfo.isCreateable());row.add(fieldInfo.isUpdateable());
  }
  System.assert(false,'OBSERVED|describeAccess|'+JSON.serialize(new Map<String,Object>{'permissions'=>row}));
 }
 @IsTest static void queryAndDml() {
  User person=actor();
  Map<String,String> row=new Map<String,String>();
  System.runAs(new User(Id=UserInfo.getUserId())) { insert new AccessNote__c(Name='Owned seed'); }
  System.runAs(person) {
   try { row.put('plainQuery',String.valueOf(Database.countQuery('SELECT COUNT() FROM AccessNote__c'))); } catch(Exception error) {row.put('plainQuery',failure(error));}
   try { row.put('systemQuery',String.valueOf(Database.countQuery('SELECT COUNT() FROM AccessNote__c WITH SYSTEM_MODE'))); } catch(Exception error) {row.put('systemQuery',failure(error));}
   try { Database.query('SELECT Id,Detail__c FROM AccessNote__c WITH USER_MODE');row.put('userQuery','ok'); } catch(Exception error) {row.put('userQuery',failure(error));}
   try { Database.query('SELECT Id,Detail__c FROM AccessNote__c WITH SECURITY_ENFORCED');row.put('enforcedQuery','ok'); } catch(Exception error) {row.put('enforcedQuery',failure(error));}
   try { insert new AccessNote__c(Name='Plain');row.put('plainInsert','ok'); } catch(Exception error) {row.put('plainInsert',failure(error));}
   try { Database.insert(new AccessNote__c(Name='User'),AccessLevel.USER_MODE);row.put('userInsert','ok'); } catch(Exception error) {row.put('userInsert',failure(error));}
   try { Database.insert(new AccessNote__c(Name='System'),AccessLevel.SYSTEM_MODE);row.put('systemInsert','ok'); } catch(Exception error) {row.put('systemInsert',failure(error));}
  }
  System.assert(false,'OBSERVED|queryAndDml|'+JSON.serialize(row));
 }
 @IsTest static void stripAccess() {
  User person=actor();
  Map<String,String> row=new Map<String,String>();
  System.runAs(person) {
   for(System.AccessType mode:new List<System.AccessType>{System.AccessType.READABLE,System.AccessType.CREATABLE,System.AccessType.UPDATABLE}) {
    try {
     SObjectAccessDecision decision=Security.stripInaccessible(mode,new List<SObject>{new AccessNote__c(Name='Strip',Detail__c='detail')},false);
     Set<String> removed=decision.getRemovedFields().get('AccessNote__c');
     row.put(String.valueOf(mode),String.valueOf(removed!=null && removed.contains('Name'))+':'+String.valueOf(removed!=null && removed.contains('Detail__c')));
    } catch(Exception error) {row.put(String.valueOf(mode),failure(error));}
   }
   try { Security.stripInaccessible(System.AccessType.READABLE,new List<SObject>{new AccessNote__c(Name='Enforced')});row.put('enforced','ok'); } catch(Exception error) {row.put('enforced',failure(error));}
  }
  System.assert(false,'OBSERVED|stripAccess|'+JSON.serialize(row));
 }
}
