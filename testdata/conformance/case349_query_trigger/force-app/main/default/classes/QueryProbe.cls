public class QueryProbe {
 public static List<Object> run() {
  Savepoint root=Database.setSavepoint();List<Object> rows=new List<Object>();
  Id pt=[SELECT Id FROM RecordType WHERE SobjectType='Account' AND IsPersonType=true WITH SYSTEM_MODE LIMIT 1].Id;
  Account seed=new Account(LastName='Synthetic',RecordTypeId=pt);Database.insert(seed,AccessLevel.SYSTEM_MODE);
  for(String op:new List<String>{'update','upsert'}) for(String mode:new List<String>{'none','same-name','changed-name','same-type','caller-name'}) {
   Savepoint sp=Database.setSavepoint();Account a=[SELECT Id,Name,RecordTypeId FROM Account WHERE Id=:seed.Id WITH SYSTEM_MODE];a.PersonHomePhone='4155550123';if(mode=='caller-name')a.Name=a.Name;
   ProbeControl.mode=mode;ProbeControl.hits=0;Boolean success=false;String status='';
   if(op=='update'){Database.SaveResult r=Database.update(a,false,AccessLevel.SYSTEM_MODE);success=r.isSuccess();status=success?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode());}
   else {Database.UpsertResult r=Database.upsert(a,false,AccessLevel.SYSTEM_MODE);success=r.isSuccess();status=success?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode());}
   Account stored=[SELECT Name,PersonHomePhone,RecordTypeId FROM Account WHERE Id=:seed.Id WITH SYSTEM_MODE];
   rows.add(new Map<String,Object>{'case'=>op+'-'+mode,'success'=>success,'status'=>status,'beforeCount'=>ProbeControl.hits,'storedName'=>stored.Name,'storedPhone'=>stored.PersonHomePhone,'sameType'=>stored.RecordTypeId==pt});Database.rollback(sp);ProbeControl.mode='none';
  }
  for(String op:new List<String>{'update','upsert'}) for(String projection:new List<String>{'name','type','both','id'}) for(String field:new List<String>{'home','mobile','email','none'}) {
   Savepoint sp=Database.setSavepoint();Account a;
   if(projection=='name')a=[SELECT Id,Name FROM Account WHERE Id=:seed.Id WITH SYSTEM_MODE];
   else if(projection=='type')a=[SELECT Id,RecordTypeId FROM Account WHERE Id=:seed.Id WITH SYSTEM_MODE];
   else if(projection=='both')a=[SELECT Id,Name,RecordTypeId FROM Account WHERE Id=:seed.Id WITH SYSTEM_MODE];
   else a=[SELECT Id FROM Account WHERE Id=:seed.Id WITH SYSTEM_MODE];
   if(field=='home')a.PersonHomePhone='4155550123';if(field=='mobile')a.PersonMobilePhone='4155550123';if(field=='email')a.PersonEmail='synthetic@example.invalid';
   ProbeControl.mode='none';ProbeControl.hits=0;Boolean success=false;String status='';List<Object> errors=new List<Object>();
   if(op=='update'){Database.SaveResult r=Database.update(a,false,AccessLevel.SYSTEM_MODE);success=r.isSuccess();status=success?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode());for(Database.Error e:r.getErrors())errors.add(new Map<String,Object>{'status'=>String.valueOf(e.getStatusCode()),'message'=>e.getMessage(),'fields'=>e.getFields()});}
   else {Database.UpsertResult r=Database.upsert(a,false,AccessLevel.SYSTEM_MODE);success=r.isSuccess();status=success?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode());for(Database.Error e:r.getErrors())errors.add(new Map<String,Object>{'status'=>String.valueOf(e.getStatusCode()),'message'=>e.getMessage(),'fields'=>e.getFields()});}
   rows.add(new Map<String,Object>{'case'=>'projection-'+op+'-'+projection+'-'+field,'success'=>success,'status'=>status,'beforeCount'=>ProbeControl.hits,'errors'=>errors});Database.rollback(sp);
  }
  Database.rollback(root);return rows;
 }
}
