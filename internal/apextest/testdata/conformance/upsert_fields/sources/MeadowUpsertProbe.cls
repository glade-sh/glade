public class MeadowUpsertProbe {
 public static Map<String,Object> observe() {
  Savepoint boundary=Database.setSavepoint();
  try {
   Map<String,Object> rows=new Map<String,Object>();
   Contact original=new Contact(LastName='Meadow',Email='statement@example.invalid',Description='Before');insert original;
   Contact replacement=new Contact(LastName='Meadow',Email=original.Email,Description='After');
   upsert replacement Contact.Fields.Email;
   rows.put('UK01',replacement.Id==original.Id && [SELECT Description FROM Contact WHERE Id=:original.Id].Description=='After');
   Contact single=new Contact(LastName='Meadow',Email='single@example.invalid',Description='Before');insert single;
   Contact nextSingle=new Contact(LastName='Meadow',Email=single.Email,Description='After');
   Database.UpsertResult singleResult=Database.upsert(nextSingle,Contact.Fields.Email,true);
   rows.put('UK02',singleResult.isSuccess() && !singleResult.isCreated() && singleResult.getId()==single.Id && [SELECT Description FROM Contact WHERE Id=:single.Id].Description=='After');
   Contact listSeed=new Contact(LastName='Meadow',Email='list@example.invalid',Description='Before');insert listSeed;
   List<Contact> input=new List<Contact>{new Contact(LastName='Meadow',Email=listSeed.Email,Description='After'),new Contact(LastName='Meadow',Email='new@example.invalid')};
   Database.UpsertResult[] results=Database.upsert(input,Contact.Fields.Email,false);
   rows.put('UK03',results.size()==2 && results[0].isSuccess() && !results[0].isCreated() && results[0].getId()==listSeed.Id && results[1].isSuccess() && results[1].isCreated() && [SELECT Description FROM Contact WHERE Id=:listSeed.Id].Description=='After');
   Contact control=new Contact(LastName='Meadow',Email='direct@example.invalid');insert control;
   Contact controlNext=new Contact(LastName='Meadow',Email=control.Email,Description='After');
   Database.UpsertResult controlResult=Database.upsert(controlNext,Contact.Email,true);
   rows.put('UC01',controlResult.isSuccess() && !controlResult.isCreated() && controlResult.getId()==control.Id && [SELECT Description FROM Contact WHERE Id=:control.Id].Description=='After');
   return rows;
  } finally { Database.rollback(boundary); }
 }
}
