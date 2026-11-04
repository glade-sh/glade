public class ProbeCases {
 public static List<Object> run() {
  Savepoint root=Database.setSavepoint(); List<Object> rows=new List<Object>();
  Account bs=new Account(Name='Business Seed'); Database.insert(bs,AccessLevel.SYSTEM_MODE);
  Id bt=[SELECT RecordTypeId FROM Account WHERE Id=:bs.Id WITH SYSTEM_MODE].RecordTypeId;
  Id pt=[SELECT Id FROM RecordType WHERE SobjectType='Account' AND IsPersonType=true WITH SYSTEM_MODE LIMIT 1].Id;
  Account seed=new Account(LastName='Synthetic',RecordTypeId=pt); Database.insert(seed,AccessLevel.SYSTEM_MODE);
  for(String operation:new List<String>{'update','upsert'}) for(String mode:new List<String>{'none','same-name','same-type','changed-name','input-name','input-type'}) {
   Savepoint sp=Database.setSavepoint(); ProbeControl.mode=mode; ProbeControl.hits=0;
   Account a=new Account(Id=seed.Id,PersonHomePhone='4155550123'); Boolean success=false; String status=''; if(mode=='input-name') a.Name='Synthetic'; if(mode=='input-type') a.RecordTypeId=pt;
   try {
    if(operation=='update') { Database.SaveResult r=Database.update(a,false,AccessLevel.SYSTEM_MODE); success=r.isSuccess(); status=success?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode()); }
    else { Database.UpsertResult r=Database.upsert(a,false,AccessLevel.SYSTEM_MODE); success=r.isSuccess(); status=success?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode()); }
    Account stored=[SELECT Name,PersonHomePhone,RecordTypeId FROM Account WHERE Id=:seed.Id WITH SYSTEM_MODE];
    rows.add(new Map<String,Object>{'case'=>operation+'-'+mode,'success'=>success,'status'=>status,'beforeCount'=>ProbeControl.hits,'storedPhone'=>stored.PersonHomePhone,'storedName'=>stored.Name,'sameType'=>stored.RecordTypeId==pt});
   } catch(Exception e) {rows.add(new Map<String,Object>{'case'=>operation+'-'+mode,'exception'=>e.getTypeName(),'beforeCount'=>ProbeControl.hits});}
   Database.rollback(sp); ProbeControl.mode='none';
  }
  for(String shape:new List<String>{'single-phone','both-phone','both-no-phone','single-no-phone'}) {
   Savepoint sp=Database.setSavepoint(); Account a=new Account(Name='Synthetic Business',RecordTypeId=bt,FirstName='Synthetic');
   if(shape.startsWith('both')) a.LastName='Person'; if(shape.endsWith('phone')&&!shape.endsWith('no-phone')) a.PersonHomePhone='4155550123';
   ProbeControl.hits=0; Database.SaveResult r=Database.insert(a,false,AccessLevel.SYSTEM_MODE);
   rows.add(new Map<String,Object>{'case'=>'business-'+shape,'success'=>r.isSuccess(),'status'=>r.isSuccess()?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode()),'beforeCount'=>ProbeControl.hits}); Database.rollback(sp);
  }
  for(String shape:new List<String>{'plain','extra','null','multiple'}) {
   Savepoint sp=Database.setSavepoint(); Account a=new Account(Name='Synthetic Business',RecordTypeId=bt,FirstName='Synthetic',ProbeKey__c='key-'+shape);
   if(shape=='extra') a.BillingCity='Synthetic'; if(shape=='null') a.put('BillingCity',null); if(shape=='multiple') {a.LastName='Person'; a.PersonTitle='Synthetic';}
   ProbeControl.hits=0; Database.UpsertResult r=Database.upsert(a,Account.Fields.ProbeKey__c,false,AccessLevel.SYSTEM_MODE);
   rows.add(new Map<String,Object>{'case'=>'external-'+shape,'success'=>r.isSuccess(),'status'=>r.isSuccess()?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode()),'created'=>r.isCreated(),'beforeCount'=>ProbeControl.hits}); Database.rollback(sp);
  }
  Database.rollback(root);return rows;
 }
}
