public class ReviewCases {
 public static List<Object> run() {
  Savepoint root=Database.setSavepoint();List<Object> rows=new List<Object>();
  Account seed=new Account(Name='Business Seed');Database.insert(seed,AccessLevel.SYSTEM_MODE);
  Id business=[SELECT RecordTypeId FROM Account WHERE Id=:seed.Id WITH SYSTEM_MODE].RecordTypeId;
  for(String kind:new List<String>{'home','mobile','email','no-contact'}) {
   Savepoint sp=Database.setSavepoint();Account a=new Account(Name='Synthetic',FirstName='Synthetic',LastName='Person',RecordTypeId=business);
   if(kind=='home') a.PersonHomePhone='4155550123';if(kind=='mobile') a.PersonMobilePhone='4155550123';if(kind=='email') a.PersonEmail='synthetic@example.invalid';
   ReviewTrace.hits=0;Database.SaveResult r=Database.insert(a,false,AccessLevel.SYSTEM_MODE);
   rows.add(new Map<String,Object>{'case'=>'mixed-'+kind,'success'=>r.isSuccess(),'status'=>r.isSuccess()?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode()),'beforeCount'=>ReviewTrace.hits});Database.rollback(sp);
  }
  for(String kind:new List<String>{'plain','city','null','multiple'}) {
   Savepoint sp=Database.setSavepoint();Id invalid=Id.valueOf('012000000000099');Account a=new Account(Name='Synthetic',FirstName='Synthetic',RecordTypeId=invalid,ProbeKey__c='key-'+kind);
   if(kind=='city') a.BillingCity='Synthetic';if(kind=='null') a.put('BillingCity',null);if(kind=='multiple'){a.LastName='Person';a.PersonTitle='Synthetic';}
   ReviewTrace.hits=0;Database.UpsertResult r=Database.upsert(a,Account.Fields.ProbeKey__c,false,AccessLevel.SYSTEM_MODE);
   List<Object> errors=new List<Object>();for(Database.Error e:r.getErrors())errors.add(new Map<String,Object>{'status'=>String.valueOf(e.getStatusCode()),'message'=>e.getMessage().replace(String.valueOf(invalid),'SELECTED_ID'),'fields'=>e.getFields()});
   rows.add(new Map<String,Object>{'case'=>'invalid-'+kind,'success'=>r.isSuccess(),'created'=>r.isCreated(),'errors'=>errors,'beforeCount'=>ReviewTrace.hits});Database.rollback(sp);
  }
  Database.rollback(root);return rows;
 }
}
