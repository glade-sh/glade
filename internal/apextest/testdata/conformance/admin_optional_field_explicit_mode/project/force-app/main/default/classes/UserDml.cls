@IsTest private class UserDml {
 static String t(Exception e) { return e.getTypeName(); }
 @IsTest static void adminDirect(){
  Map<String,Object> o=new Map<String,Object>();
  CapObj__c base=new CapObj__c(Name='base',Required__c='r'); Database.insert(base,AccessLevel.SYSTEM_MODE);
  try { Database.insert(new CapObj__c(Name='a',Required__c='r',Optional__c='x'),AccessLevel.USER_MODE); o.put('userInsert','ok'); } catch(Exception e) {o.put('userInsert',t(e)+':'+e.getMessage());}
  try { Database.update(new CapObj__c(Id=base.Id,Optional__c='z'),AccessLevel.USER_MODE); o.put('userUpdate','ok'); } catch(Exception e) {o.put('userUpdate',t(e)+':'+e.getMessage());}
  try { Database.SaveResult r=Database.insert(new CapObj__c(Name='b',Required__c='r',Optional__c='x'),false,AccessLevel.USER_MODE); List<String> errs=new List<String>(); for(Database.Error er:r.getErrors()) errs.add(String.valueOf(er.getStatusCode())+'/'+er.getFields()); o.put('userInsertFalse',new List<Object>{r.isSuccess(),errs}); } catch(Exception e) {o.put('userInsertFalse',t(e)+':'+e.getMessage());}
  try { Database.SaveResult r=Database.insert(new CapObj__c(Name='c',Required__c='r',Optional__c='x'),false,AccessLevel.SYSTEM_MODE); o.put('sysInsertFalse',r.isSuccess()); } catch(Exception e) {o.put('sysInsertFalse',t(e)+':'+e.getMessage());}
  try { List<CapObj__c> l=Database.query('SELECT Id, Optional__c FROM CapObj__c',AccessLevel.USER_MODE); o.put('dynUserMode','ok:'+l.size()); } catch(Exception e) {o.put('dynUserMode',t(e)+':'+e.getMessage());}
  try { List<CapObj__c> l=Database.query('SELECT Id, Optional__c FROM CapObj__c',AccessLevel.SYSTEM_MODE); o.put('dynSystemMode','ok:'+l.size()); } catch(Exception e) {o.put('dynSystemMode',t(e)+':'+e.getMessage());}
  System.assert(false,'OBSERVED|adminDirect|'+JSON.serialize(o));
 }
}
