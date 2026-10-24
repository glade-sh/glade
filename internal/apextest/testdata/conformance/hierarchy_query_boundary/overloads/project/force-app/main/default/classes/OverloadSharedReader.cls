public with sharing class OverloadSharedReader {
 public static Map<String,Object> read() {
  Id owner=UserInfo.getOrganizationId();
  String q='SELECT Disabled__c FROM BoundaryPreference__c WHERE SetupOwnerId=:owner';
  String c='SELECT COUNT() FROM BoundaryPreference__c WHERE SetupOwnerId=:owner';
  Map<String,Object> b=new Map<String,Object>{'owner'=>owner};
  Map<String,Object> result=new Map<String,Object>();
  try { result.put('queryDefault',Database.query(q).size()); } catch(Exception e) { result.put('queryDefault',e.getTypeName()); }
  try { result.put('queryUser',Database.query(q,AccessLevel.USER_MODE).size()); } catch(Exception e) { result.put('queryUser',e.getTypeName()); }
  try { result.put('querySystem',Database.query(q,AccessLevel.SYSTEM_MODE).size()); } catch(Exception e) { result.put('querySystem',e.getTypeName()); }
  try { result.put('queryBindsUser',Database.queryWithBinds(q,b,AccessLevel.USER_MODE).size()); } catch(Exception e) { result.put('queryBindsUser',e.getTypeName()); }
  try { result.put('queryBindsSystem',Database.queryWithBinds(q,b,AccessLevel.SYSTEM_MODE).size()); } catch(Exception e) { result.put('queryBindsSystem',e.getTypeName()); }
  try { result.put('countDefault',Database.countQuery(c)); } catch(Exception e) { result.put('countDefault',e.getTypeName()); }
  try { result.put('countUser',Database.countQuery(c,AccessLevel.USER_MODE)); } catch(Exception e) { result.put('countUser',e.getTypeName()); }
  try { result.put('countSystem',Database.countQuery(c,AccessLevel.SYSTEM_MODE)); } catch(Exception e) { result.put('countSystem',e.getTypeName()); }
  try { result.put('countBindsUser',Database.countQueryWithBinds(c,b,AccessLevel.USER_MODE)); } catch(Exception e) { result.put('countBindsUser',e.getTypeName()); }
  try { result.put('countBindsSystem',Database.countQueryWithBinds(c,b,AccessLevel.SYSTEM_MODE)); } catch(Exception e) { result.put('countBindsSystem',e.getTypeName()); }
  return result;
 }
}
