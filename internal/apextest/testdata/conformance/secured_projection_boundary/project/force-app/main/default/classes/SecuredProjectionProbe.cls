public class SecuredProjectionProbe {
 public static Map<String,Object> observe() {
  if (!Test.isRunningTest()) { return new Map<String,Object>{'route'=>'anonymous control'}; }
  User actor=new User(Alias='secure',Email='secure@example.invalid',Username='secure.'+UserInfo.getOrganizationId()+'@example.invalid',ProfileId=[SELECT Id FROM Profile WHERE Name='Minimum Access - Salesforce' LIMIT 1].Id,TimeZoneSidKey='America/Los_Angeles',LocaleSidKey='en_US',EmailEncodingKey='UTF-8',LanguageLocaleKey='en_US',LastName='Secure');
  insert actor;
  PermissionSet permissions=new PermissionSet(Name='OwnedProjection',Label='Owned projection'); insert permissions;
  insert new List<ObjectPermissions>{new ObjectPermissions(ParentId=permissions.Id,SObjectType='BatchItem__c',PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=true),new ObjectPermissions(ParentId=permissions.Id,SObjectType='BatchNote__c',PermissionsRead=true,PermissionsCreate=true,PermissionsEdit=true)};
  insert new List<FieldPermissions>{new FieldPermissions(ParentId=permissions.Id,SObjectType='BatchItem__c',Field='BatchItem__c.Stage__c',PermissionsRead=true,PermissionsEdit=true),new FieldPermissions(ParentId=permissions.Id,SObjectType='BatchNote__c',Field='BatchNote__c.BatchItem__c',PermissionsRead=true,PermissionsEdit=true)};
  insert new PermissionSetAssignment(AssigneeId=actor.Id,PermissionSetId=permissions.Id);
  Map<String,Object> observed;
  System.runAs(actor) { observed=readRows(); }
  return observed;
 }
 private static Map<String,Object> readRows() {
  Map<String,Object> out=new Map<String,Object>();
  BatchItem__c parent=new BatchItem__c(Name='Owned item',Stage__c='Ready'); insert parent;
  insert new BatchNote__c(Name='Owned note',BatchItem__c=parent.Id);
  Id parentId=parent.Id;
  try {
   List<SObject> groups=Database.query('SELECT Stage__c stageKey, COUNT(Id) tally FROM BatchItem__c WHERE Id = :parentId GROUP BY Stage__c',AccessLevel.USER_MODE);
   out.put('aliasCount',groups.size()); out.put('aliasValue',groups[0].get('stageKey')); out.put('aliasTally',groups[0].get('tally'));
  } catch(Exception e) { out.put('aliasError',e.getTypeName()+':'+e.getMessage()); }
  BatchItem__c queried=[SELECT Id,Name,Stage__c,(SELECT Id FROM Notes__r) FROM BatchItem__c WHERE Id=:parentId];
  queried.Name='Owned revised';
  try {
   List<Database.UpsertResult> result=Database.upsert(new List<BatchItem__c>{queried},AccessLevel.USER_MODE);
   out.put('childQueryUpsert',result[0].isSuccess()); out.put('updatedName',[SELECT Name FROM BatchItem__c WHERE Id=:parentId].Name);
  } catch(Exception e) { out.put('childQueryError',e.getTypeName()+':'+e.getMessage()); }
  try {
   List<SObject> hidden=Database.query('SELECT Hidden__c hiddenKey, COUNT(Id) tally FROM BatchItem__c WHERE Id = :parentId GROUP BY Hidden__c',AccessLevel.USER_MODE);
   out.put('hiddenQuery','returned');
  } catch(Exception e) { out.put('hiddenQueryError',e.getTypeName()+':'+e.getMessage()); }
  BatchItem__c denied=new BatchItem__c(Id=parentId,Hidden__c='owned hidden');
  try {
   List<Database.UpsertResult> hiddenResult=Database.upsert(new List<BatchItem__c>{denied},AccessLevel.USER_MODE);
   out.put('hiddenWrite','returned');
  } catch(Exception e) { out.put('hiddenWriteError',e.getTypeName()+':'+e.getMessage()); }
  return out;
 }
}
