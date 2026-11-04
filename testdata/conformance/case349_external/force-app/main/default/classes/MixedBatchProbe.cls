public class MixedBatchProbe {
 public static List<Object> run() {
  Savepoint rootPoint=Database.setSavepoint(); List<Object> rows=new List<Object>();
  Account first=new Account(Name='First seed',ProbeKey__c='batch-first'); Account second=new Account(Name='Second seed',ProbeKey__c='batch-second');
  Database.insert(new List<Account>{first,second},AccessLevel.SYSTEM_MODE);
  Id rt=[SELECT RecordTypeId FROM Account WHERE Id=:first.Id WITH SYSTEM_MODE].RecordTypeId;
  for(String op:new List<String>{'insert','update','upsert'}) {
   Savepoint sp=Database.setSavepoint(); Account a; Account b;
   if(op=='insert') { a=new Account(Name='First',PersonEmail='invalid'); b=new Account(Name='Second',RecordTypeId=rt,FirstName='Synthetic'); }
   else if(op=='update') { a=new Account(Id=first.Id,PersonEmail='invalid'); b=new Account(Id=second.Id,FirstName='Synthetic'); }
   else { a=new Account(ProbeKey__c='batch-first',PersonEmail='invalid'); b=new Account(ProbeKey__c='batch-second',FirstName='Synthetic'); }
   ExternalTrace.beforeValues.clear();
   try {
    if(op=='insert') Database.insert(new List<Account>{a,b},true,AccessLevel.SYSTEM_MODE);
    else if(op=='update') Database.update(new List<Account>{a,b},true,AccessLevel.SYSTEM_MODE);
    else Database.upsert(new List<Account>{a,b},Account.Fields.ProbeKey__c,true,AccessLevel.SYSTEM_MODE);
    rows.add(new Map<String,Object>{'case'=>op,'outcome'=>'success','before'=>ExternalTrace.beforeValues.clone()});
   } catch(DmlException e) {
    List<String> statuses=new List<String>(); List<Integer> indices=new List<Integer>();
    for(Integer i=0;i<e.getNumDml();i++) { statuses.add(String.valueOf(e.getDmlStatusCode(i))); indices.add(e.getDmlIndex(i)); }
    rows.add(new Map<String,Object>{'case'=>op,'outcome'=>'exception','statuses'=>statuses,'indices'=>indices,'before'=>ExternalTrace.beforeValues.clone()});
   }
   Database.rollback(sp);
  }
  Database.rollback(rootPoint);return rows;
 }
}
