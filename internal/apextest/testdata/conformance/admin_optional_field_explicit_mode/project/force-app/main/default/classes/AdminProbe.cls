@IsTest private class AdminProbe {
 static String t(Exception e) { return e.getTypeName(); }
 static Map<String,Object> observe() {
  Map<String,Object> o=new Map<String,Object>();
  Schema.DescribeFieldResult f=CapObj__c.Optional__c.getDescribe(); Schema.DescribeSObjectResult s=CapObj__c.SObjectType.getDescribe();
  o.put('describe',new List<Boolean>{s.isAccessible(),s.isCreateable(),f.isAccessible(),f.isCreateable(),f.isUpdateable()});
  CapObj__c base=new CapObj__c(Name='base',Required__c='r'); Database.insert(base,AccessLevel.SYSTEM_MODE);
  try { insert new CapObj__c(Name='a',Required__c='r',Optional__c='x'); o.put('insert','ok'); } catch(Exception e) {o.put('insert',t(e)+':'+e.getMessage());}
  try { base.Optional__c='y'; update base; o.put('update','ok'); } catch(Exception e) {o.put('update',t(e)+':'+e.getMessage());}
  try { Database.SaveResult r=Database.insert(new CapObj__c(Name='b',Required__c='r',Optional__c='x'),false); List<String> errs=new List<String>(); for(Database.Error er:r.getErrors()) errs.add(String.valueOf(er.getStatusCode())+'/'+er.getFields()); o.put('dbInsertFalse',new List<Object>{r.isSuccess(),errs}); } catch(Exception e) {o.put('dbInsertFalse',t(e)+':'+e.getMessage());}
  try { Database.SaveResult r=Database.update(new CapObj__c(Id=base.Id,Optional__c='z'),false); List<String> errs=new List<String>(); for(Database.Error er:r.getErrors()) errs.add(String.valueOf(er.getStatusCode())+'/'+er.getFields()); o.put('dbUpdateFalse',new List<Object>{r.isSuccess(),errs}); } catch(Exception e) {o.put('dbUpdateFalse',t(e)+':'+e.getMessage());}
  try { List<CapObj__c> l=[SELECT Id, Optional__c FROM CapObj__c]; o.put('soql','ok:'+l.size()); } catch(Exception e) {o.put('soql',t(e)+':'+e.getMessage());}
  try { List<CapObj__c> l=[SELECT Id, Optional__c FROM CapObj__c WITH USER_MODE]; o.put('soqlUserMode','ok:'+l.size()); } catch(Exception e) {o.put('soqlUserMode',t(e)+':'+e.getMessage());}
  try { List<CapObj__c> l=(List<CapObj__c>)Database.query('SELECT Id, Optional__c FROM CapObj__c WITH SECURITY_ENFORCED'); o.put('soqlEnforced','ok:'+l.size()); } catch(Exception e) {o.put('soqlEnforced',t(e)+':'+e.getMessage());}
  try { List<CapObj__c> l=[SELECT Id, Optional__c FROM CapObj__c WITH SYSTEM_MODE]; o.put('soqlSystemMode','ok:'+l.size()); } catch(Exception e) {o.put('soqlSystemMode',t(e)+':'+e.getMessage());}
  return o;
 }
 @IsTest static void adminDirect(){ Map<String,Object> row=observe(); System.assert(false,'OBSERVED|adminDirect|'+JSON.serialize(row)); }
}
