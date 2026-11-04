public class DetailProbe {
 public static List<Object> run() {
  Savepoint root=Database.setSavepoint();List<Object> rows=new List<Object>();
  Account b=new Account(Name='Business Seed');Database.insert(b,AccessLevel.SYSTEM_MODE);
  Id bt=[SELECT RecordTypeId FROM Account WHERE Id=:b.Id WITH SYSTEM_MODE].RecordTypeId;
  Id pt=[SELECT Id FROM RecordType WHERE SobjectType='Account' AND IsPersonType=true WITH SYSTEM_MODE LIMIT 1].Id;
  for(String kind:new List<String>{'business','person','invalid'}) {
   Id selected=kind=='business'?bt:kind=='person'?pt:Id.valueOf('012000000000099');
   Account a=new Account(Name='Synthetic Business',FirstName='Synthetic',RecordTypeId=selected,ProbeKey__c='key-'+kind);
   Database.UpsertResult r=Database.upsert(a,Account.Fields.ProbeKey__c,false,AccessLevel.SYSTEM_MODE);
   List<Object> errors=new List<Object>();for(Database.Error e:r.getErrors()) errors.add(new Map<String,Object>{'status'=>String.valueOf(e.getStatusCode()),'message'=>e.getMessage().replace(String.valueOf(selected),'SELECTED_ID'),'fields'=>e.getFields()});
   rows.add(new Map<String,Object>{'case'=>kind,'success'=>r.isSuccess(),'created'=>r.isCreated(),'errors'=>errors});
  }
  Database.rollback(root);return rows;
 }
}
