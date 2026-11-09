@IsTest private class PermissionProbeTest {
 static User standardActor() {
  User u=new User(Alias='probe',Email='probe@example.invalid',EmailEncodingKey='UTF-8',LastName='Probe',LanguageLocaleKey='en_US',LocaleSidKey='en_US',TimeZoneSidKey='America/Los_Angeles',Username='probe-'+UserInfo.getOrganizationId()+'@example.invalid',ProfileId=[SELECT Id FROM Profile WHERE Name='Standard User'].Id); insert u; return u;
 }
 static String result(Exception e) { return e.getTypeName(); }
 static Map<String,Object> observe() {
  Map<String,Object> out=new Map<String,Object>();
  Schema.DescribeSObjectResult o=AccessNote__c.SObjectType.getDescribe(); Schema.DescribeFieldResult f=AccessNote__c.Detail__c.getDescribe();
  out.put('describe',new List<Boolean>{o.isAccessible(),o.isCreateable(),o.isUpdateable(),o.isDeletable(),f.isAccessible(),f.isCreateable(),f.isUpdateable()});
  try { Database.query('SELECT Id, Detail__c FROM AccessNote__c WITH USER_MODE'); out.put('userQuery','ok'); } catch(Exception e) {out.put('userQuery',result(e));}
  try { Database.query('SELECT Id, Detail__c FROM AccessNote__c WITH SECURITY_ENFORCED'); out.put('enforcedQuery','ok'); } catch(Exception e) {out.put('enforcedQuery',result(e));}
  try { Database.insert(new AccessNote__c(Name='User',Detail__c='text'),AccessLevel.USER_MODE);out.put('userInsert','ok'); } catch(Exception e) {out.put('userInsert',result(e));}
  try { Database.insert(new AccessNote__c(Name='System',Detail__c='text'),AccessLevel.SYSTEM_MODE);out.put('systemInsert','ok'); } catch(Exception e) {out.put('systemInsert',result(e));}
  try { SObjectAccessDecision d=Security.stripInaccessible(AccessType.CREATABLE,new List<SObject>{new AccessNote__c(Name='Strip',Detail__c='text')},false); Set<String> removed=d.getRemovedFields().get('AccessNote__c');out.put('strip',new List<Boolean>{removed!=null && removed.contains('Name'),removed!=null && removed.contains('Detail__c')}); } catch(Exception e) {out.put('strip',result(e));}
  return out;
 }
 @IsTest static void adminDirect(){ Map<String,Object> row=observe();System.assert(false,'OBSERVED|adminDirect|'+JSON.serialize(row)); }
 @IsTest static void adminRunAs(){ Map<String,Object> row;System.runAs(new User(Id=UserInfo.getUserId())) {row=observe();} System.assert(false,'OBSERVED|adminRunAs|'+JSON.serialize(row)); }
 @IsTest static void standardRunAs(){ User u=standardActor();Map<String,Object> row;System.runAs(u) {row=observe();} System.assert(false,'OBSERVED|standardRunAs|'+JSON.serialize(row)); }
}
