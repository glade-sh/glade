public class ReviewMixedProbe {
 public static List<Object> run() {
  Savepoint sp=Database.setSavepoint(); List<Object> rows=new List<Object>();
  Account b=new Account(Name='Business seed'); Database.insert(b,AccessLevel.SYSTEM_MODE);
  Id business=[SELECT RecordTypeId FROM Account WHERE Id=:b.Id WITH SYSTEM_MODE].RecordTypeId;
  Id person=[SELECT Id FROM RecordType WHERE SobjectType='Account' AND IsPersonType=true LIMIT 1].Id;
  Account seed=new Account(LastName='Synthetic',RecordTypeId=person); Database.insert(seed,AccessLevel.SYSTEM_MODE);
  for(String kind:new List<String>{'defaultCustomPhone','defaultCustomOnly','businessTitlePhone','businessFirstPhone','personNamePhone','personPhone'}) {
   Account a;
   if(kind=='personNamePhone' || kind=='personPhone') { a=new Account(Id=seed.Id,PersonHomePhone='4155550123'); if(kind=='personNamePhone') a.Name='Synthetic'; }
   else { a=new Account(Name='Business candidate'); if(kind=='defaultCustomPhone'||kind=='defaultCustomOnly') a.PersonMemo__c='synthetic'; if(kind=='defaultCustomPhone'||kind=='businessTitlePhone'||kind=='businessFirstPhone') a.PersonHomePhone='4155550123'; if(kind=='businessTitlePhone'||kind=='businessFirstPhone') a.RecordTypeId=business; if(kind=='businessTitlePhone') a.PersonTitle='Synthetic'; if(kind=='businessFirstPhone') a.FirstName='Synthetic'; }
   ReviewRouteTrace.kinds.clear(); Database.SaveResult result;
   try {
    if(kind=='personNamePhone'||kind=='personPhone') result=Database.update(a,false,AccessLevel.SYSTEM_MODE); else result=Database.insert(a,false,AccessLevel.SYSTEM_MODE);
    rows.add(new Map<String,Object>{'case'=>kind,'success'=>result.isSuccess(),'status'=>result.isSuccess()?'SUCCESS':String.valueOf(result.getErrors()[0].getStatusCode()),'before'=>ReviewRouteTrace.kinds.clone()});
   } catch(Exception e) { rows.add(new Map<String,Object>{'case'=>kind,'exception'=>e.getTypeName(),'message'=>e.getMessage(),'before'=>ReviewRouteTrace.kinds.clone()}); }
  }
  Database.rollback(sp);return rows;
 }
}
