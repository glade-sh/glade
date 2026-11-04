public class ComponentProbe {
 public static List<Object> run() {
  Savepoint root=Database.setSavepoint(); List<Object> rows=new List<Object>();
  Id pt=[SELECT Id FROM RecordType WHERE SobjectType='Account' AND IsPersonType=true WITH SYSTEM_MODE LIMIT 1].Id;
  Account seed=new Account(FirstName='Initial',LastName='Synthetic',RecordTypeId=pt); Database.insert(seed,AccessLevel.SYSTEM_MODE);
  for(String op:new List<String>{'update','upsert'}) for(String projection:new List<String>{'name','id'}) for(String field:new List<String>{'FirstName','LastName'}) {
   Savepoint sp=Database.setSavepoint(); Account a;
   if(projection=='name') a=[SELECT Id,Name FROM Account WHERE Id=:seed.Id WITH SYSTEM_MODE];
   else a=[SELECT Id FROM Account WHERE Id=:seed.Id WITH SYSTEM_MODE];
   a.put(field,'Changed'); ComponentControl.hits=0;
   Boolean success=false; String status=''; List<Object> errors=new List<Object>();
   if(op=='update'){Database.SaveResult r=Database.update(a,false,AccessLevel.SYSTEM_MODE);success=r.isSuccess();status=success?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode());for(Database.Error e:r.getErrors())errors.add(new Map<String,Object>{'status'=>String.valueOf(e.getStatusCode()),'message'=>e.getMessage(),'fields'=>e.getFields()});}
   else {Database.UpsertResult r=Database.upsert(a,false,AccessLevel.SYSTEM_MODE);success=r.isSuccess();status=success?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode());for(Database.Error e:r.getErrors())errors.add(new Map<String,Object>{'status'=>String.valueOf(e.getStatusCode()),'message'=>e.getMessage(),'fields'=>e.getFields()});}
   Account stored=[SELECT FirstName,LastName,Name FROM Account WHERE Id=:seed.Id WITH SYSTEM_MODE];
   rows.add(new Map<String,Object>{'case'=>op+'-'+projection+'-'+field,'success'=>success,'status'=>status,'beforeCount'=>ComponentControl.hits,'errors'=>errors,'storedFirst'=>stored.FirstName,'storedLast'=>stored.LastName,'storedName'=>stored.Name}); Database.rollback(sp);
  }
  Database.rollback(root); return rows;
 }
}
