public without sharing class BoundarySystemReader {
 public static Map<String,Object> read() {
  Map<String,Object> result=new Map<String,Object>();
  try { result.put('instance',BoundaryPreference__c.getInstance().Disabled__c); } catch(Exception e) { result.put('instanceError',e.getTypeName()); }
  try { result.put('defaults',BoundaryPreference__c.getOrgDefaults().Disabled__c); } catch(Exception e) { result.put('defaultsError',e.getTypeName()); }
  try { BoundaryPreference__c value=BoundaryPreference__c.getValues(UserInfo.getUserId());result.put('values',value==null?null:value.Disabled__c); } catch(Exception e) { result.put('valuesError',e.getTypeName()); }
  Id orgId=UserInfo.getOrganizationId();
  try { result.put('staticRows',[SELECT Disabled__c FROM BoundaryPreference__c WHERE SetupOwnerId=:orgId].size()); } catch(Exception e) { result.put('staticError',e.getTypeName());result.put('staticMessage',e.getMessage()); }
  try { result.put('dynamicRows',Database.query('SELECT Disabled__c FROM BoundaryPreference__c WHERE SetupOwnerId=:orgId').size()); } catch(Exception e) { result.put('dynamicError',e.getTypeName()); }
  try { result.put('countRows',[SELECT COUNT() FROM BoundaryPreference__c WHERE SetupOwnerId=:orgId]); } catch(Exception e) { result.put('countError',e.getTypeName()); }
  try { result.put('systemRows',[SELECT Disabled__c FROM BoundaryPreference__c WHERE SetupOwnerId=:orgId WITH SYSTEM_MODE].size()); } catch(Exception e) { result.put('systemError',e.getTypeName()); }
  try { result.put('userRows',[SELECT Disabled__c FROM BoundaryPreference__c WHERE SetupOwnerId=:orgId WITH USER_MODE].size()); } catch(Exception e) { result.put('userError',e.getTypeName()); }
  try { result.put('boundRows',Database.queryWithBinds('SELECT Disabled__c FROM BoundaryPreference__c WHERE SetupOwnerId=:owner',new Map<String,Object>{'owner'=>orgId},AccessLevel.USER_MODE).size()); } catch(Exception e) { result.put('boundError',e.getTypeName()); }
  return result;
 }
}
