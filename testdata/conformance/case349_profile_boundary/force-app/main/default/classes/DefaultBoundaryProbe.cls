public class DefaultBoundaryProbe {
 public static String run() {
  Savepoint root = Database.setSavepoint();
  List<Object> rows=new List<Object>();
  Id p=[SELECT Id FROM RecordType WHERE SobjectType='Account' AND DeveloperName='Provider' LIMIT 1].Id;
  Id b=[SELECT Id FROM RecordType WHERE SobjectType='Account' AND DeveloperName='Business_Default' LIMIT 1].Id;
  Map<String,Object> ds=new Map<String,Object>();
  for(Schema.RecordTypeInfo i:Schema.SObjectType.Account.getRecordTypeInfos()) if(i.isDefaultRecordTypeMapping()) ds.put(i.getDeveloperName(),i.isAvailable());
  for(String n:new List<String>{'omitted','person','business','invalid','last','title'}) {
   Savepoint sp=Database.setSavepoint();
   Account a=new Account(ProbeKey__c='test-'+n,Name='Synthetic',FirstName='Synthetic');
   if(n=='person') a.RecordTypeId=p;
   if(n=='business') a.RecordTypeId=b;
   if(n=='invalid') a.RecordTypeId=Id.valueOf('012000000000099');
   if(n=='last') a.LastName='Person';
   if(n=='title') a.PersonTitle='Synthetic';
   ExternalTrace.beforeValues.clear();
   try { Database.UpsertResult r=Database.upsert(a,Account.Fields.ProbeKey__c,false,AccessLevel.SYSTEM_MODE); rows.add(new Map<String,Object>{'case'=>n,'success'=>r.isSuccess(),'created'=>r.isCreated(),'status'=>r.isSuccess()?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode()),'before'=>ExternalTrace.beforeValues.clone()}); }
   catch(Exception e){rows.add(new Map<String,Object>{'case'=>n,'exception'=>e.getTypeName(),'before'=>ExternalTrace.beforeValues.clone()});}
   Database.rollback(sp);
  }
  for(String f:new List<String>{'PersonHomePhone','PersonMobilePhone','PersonEmail'}) for(String v:new List<String>{null,'value'}) {
   Savepoint sp=Database.setSavepoint(); Account a=new Account(Name='Synthetic'); a.put(f,v==null?null:(f=='PersonEmail'?'synthetic@example.invalid':'4155550123')); ExternalTrace.beforeValues.clear();
   try { Database.SaveResult r=Database.insert(a,false,AccessLevel.SYSTEM_MODE); rows.add(new Map<String,Object>{'case'=>f+'-'+(v==null?'null':'value'),'success'=>r.isSuccess(),'status'=>r.isSuccess()?'SUCCESS':String.valueOf(r.getErrors()[0].getStatusCode()),'before'=>ExternalTrace.beforeValues.clone()}); }
   catch(Exception e){rows.add(new Map<String,Object>{'case'=>f+'-'+v,'exception'=>e.getTypeName(),'before'=>ExternalTrace.beforeValues.clone()});} Database.rollback(sp);
  }
  Database.rollback(root); return JSON.serialize(new Map<String,Object>{'defaults'=>ds,'rows'=>rows});
 }
}
