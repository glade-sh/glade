public class ExternalProbe {
 public static List<Object> run() {
  Savepoint rootPoint=Database.setSavepoint();
  Id personType=[SELECT Id FROM RecordType WHERE SobjectType='Account' AND IsPersonType=true LIMIT 1].Id;
  Account b=new Account(Name='Business seed',ProbeKey__c='business'); Database.insert(b,AccessLevel.SYSTEM_MODE);
  Account p=new Account(LastName='Person seed',RecordTypeId=personType,ProbeKey__c='person'); Database.insert(p,AccessLevel.SYSTEM_MODE);
  List<Object> rows=new List<Object>();
  for(String name:new List<String>{'personTypePhone','personTypeFirst','personTypeName','businessFirst','businessBadEmail','businessValidEmail','businessBlankEmail','personName','personPhone','unmatchedFirst','unmatchedBadEmail','unmatchedValidEmail','unmatchedBlankEmail'}) {
   Savepoint sp=Database.setSavepoint();
   Boolean explicitType=name.startsWith('personType');
   Boolean isPerson=explicitType || name.startsWith('person');
   Boolean unmatched=name.startsWith('unmatched');
   Account a=new Account();
   if(explicitType) { a.Id=p.Id; a.RecordTypeId=personType; }
   else { a.ProbeKey__c=unmatched?'unmatched':(isPerson?'person':'business'); if(unmatched) a.Name='Unmatched'; }
   if(name.endsWith('First')) a.FirstName='Synthetic';
   if(name.endsWith('Name')) { a.Name='Synthetic'; a.PersonHomePhone='4155550123'; }
   if(name.endsWith('Phone')) a.PersonHomePhone='4155550123';
   if(name.endsWith('BadEmail')) a.PersonEmail='invalid';
   if(name.endsWith('ValidEmail')) a.PersonEmail='synthetic@example.invalid';
   if(name.endsWith('BlankEmail')) a.PersonEmail='   ';
   ExternalTrace.beforeValues.clear();
   try {
    Boolean success; Boolean created=false; List<Database.Error> errors;
    if(explicitType) { Database.SaveResult r=Database.update(a,false,AccessLevel.SYSTEM_MODE); success=r.isSuccess(); errors=r.getErrors(); }
    else { Database.UpsertResult r=Database.upsert(a,Account.Fields.ProbeKey__c,false,AccessLevel.SYSTEM_MODE); success=r.isSuccess(); created=r.isCreated(); errors=r.getErrors(); }
    rows.add(new Map<String,Object>{'case'=>name,'success'=>success,'created'=>created,'status'=>success?'SUCCESS':String.valueOf(errors[0].getStatusCode()),'fields'=>success?new List<String>():errors[0].getFields(),'before'=>ExternalTrace.beforeValues.clone()});
   } catch(Exception e) { rows.add(new Map<String,Object>{'case'=>name,'exception'=>e.getTypeName(),'message'=>e.getMessage(),'before'=>ExternalTrace.beforeValues.clone()}); }
   Database.rollback(sp);
  }
  Database.rollback(rootPoint); return rows;
 }
}
