public class LocaleRoutesProbe {
 static Integer sequence = 0;
 static User makeUser(String value) {
  sequence++;
  User current = [SELECT ProfileId,TimeZoneSidKey,LanguageLocaleKey,EmailEncodingKey FROM User WHERE Id=:UserInfo.getUserId()];
  return new User(Alias='locprobe', LastName='Locale Probe', Email='locale@example.invalid',
   IsActive=false, Username='locale'+sequence+String.valueOf(Datetime.now().getTime())+'@example.invalid',
   ProfileId=current.ProfileId, TimeZoneSidKey=current.TimeZoneSidKey,
   LanguageLocaleKey=current.LanguageLocaleKey, EmailEncodingKey=current.EmailEncodingKey, LocaleSidKey=value);
 }
 static Map<String,Object> problem(Database.Error e) {
  return new Map<String,Object>{'success'=>false,'status'=>String.valueOf(e.getStatusCode()),'message'=>e.getMessage(),'fields'=>e.getFields()};
 }
 static Map<String,Object> statementProblem(DmlException e) {
  List<String> fields=new List<String>();
  if (e.getDmlFields(0) != null) for (Schema.SObjectField f : e.getDmlFields(0)) fields.add(f.getDescribe().getName());
  return new Map<String,Object>{'success'=>false,'status'=>String.valueOf(e.getDmlType(0)),'message'=>e.getDmlMessage(0),'fields'=>fields};
 }
 public static Map<String,Object> observe() {
  Map<String,Object> result = new Map<String,Object>();
  for (String route : new List<String>{'insert','update','upsertInsert','upsertUpdate'}) {
   for (String kind : new List<String>{'statement','database'}) {
    for (String value : new List<String>{'en_US','af','az','NotAnAllowedValue'}) {
     SObject row = (route == 'update' || route == 'upsertUpdate') ? new User(Id=UserInfo.getUserId(),LocaleSidKey=value) : makeUser(value);
     Map<String,Object> output = new Map<String,Object>{'success'=>true};
     if (kind == 'statement') {
      try {
       if (route == 'insert') insert row;
       else if (route == 'update') update row;
       else upsert row;
      } catch (DmlException e) { output = statementProblem(e); }
     } else if (route == 'insert') {
      Database.SaveResult r=Database.insert(row,false); if (!r.isSuccess()) output=problem(r.getErrors()[0]);
     } else if (route == 'update') {
      Database.SaveResult r=Database.update(row,false); if (!r.isSuccess()) output=problem(r.getErrors()[0]);
     } else {
      Database.UpsertResult r=Database.upsert(row,false); if (!r.isSuccess()) output=problem(r.getErrors()[0]);
     }
     result.put(route+'.'+kind+'.'+value,output);
    }
   }
  }
  return result;
 }
}
