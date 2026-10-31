public class PersonBoundaryReview {
 public static List<String> observe() {
  List<String> rows=new List<String>();
  Id business=[SELECT Id FROM RecordType WHERE SobjectType='Account' AND DeveloperName='Business_Default' LIMIT 1].Id;
  Id person=[SELECT Id FROM RecordType WHERE SobjectType='Account' AND DeveloperName='Patient' LIMIT 1].Id;
  Account seed=new Account(Name='Seed business',RecordTypeId=business); insert seed;
  for(String route:new List<String>{'database','statement_insert','statement_upsert','list_insert','list_upsert'}) {
   for(String shape:new List<String>{'null_no_type','null_business_type','null_business_missing','null_person_type','omitted_business'}) {
    Account a=new Account();
    if(shape!='omitted_business') a.LastName=null;
    if(shape=='null_business_type' || shape=='omitted_business') a.Name='Business';
    if(shape=='null_business_type' || shape=='null_business_missing' || shape=='omitted_business') a.RecordTypeId=business;
    if(shape=='null_person_type') { a.FirstName='Person'; a.RecordTypeId=person; }
    String out='success';
    try {
     if(route=='database') { Database.SaveResult r=Database.insert(a,false); if(!r.isSuccess()) out=errorOf(r.getErrors()[0]); }
     else if(route=='statement_insert') { insert a; }
     else if(route=='statement_upsert') { upsert a; }
     else if(route=='list_insert') { List<Account> batch=new List<Account>{a}; insert batch; }
     else { List<Account> batch=new List<Account>{a}; upsert batch; }
    } catch(DmlException e) { out=String.valueOf(e.getDmlStatusCode(0))+':'+String.join(e.getDmlFieldNames(0),','); }
    catch(Exception e) { out=e.getTypeName()+':'+e.getMessage(); }
    rows.add(route+'_'+shape+'|'+out);
   }
  }
  for(String route:new List<String>{'user_new_scalar','user_new_list','user_new_selector','user_existing_scalar','user_existing_list','user_existing_selector','user_mixed','api_new','api_new_list','api_user','api_system','api_user_list','api_system_list','api_selector','api_selector_list','api_existing_upsert','api_existing_update'}) {
   Account a=new Account(Name='New business',RecordTypeId=business); Account b=new Account(Name='Second business',RecordTypeId=business);
   if(route.startsWith('user_existing') || route.startsWith('api_existing')) a.Id=seed.Id;
   if(route=='user_existing_list' || route=='user_mixed') b.Id=seed.Id;
   Integer statements=Limits.getDmlStatements(), count=Limits.getDmlRows(); String out='success';
   try {
    if(route=='user_new_scalar' || route=='user_existing_scalar') { upsert as user a; }
    else if(route=='user_new_selector' || route=='user_existing_selector') { upsert as user a Account.Id; }
    else if(route.startsWith('user_')) { List<Account> batch=route=='user_existing_list' ? new List<Account>{a} : new List<Account>{a,b}; upsert as user batch; }
    else if(route=='api_new_list') { Database.UpsertResult r=Database.upsert(new List<Account>{a,b},false)[0]; if(!r.isSuccess()) out=errorOf(r.getErrors()[0]); }
    else if(route=='api_user_list') { Database.UpsertResult r=Database.upsert(new List<Account>{a,b},false,AccessLevel.USER_MODE)[0]; if(!r.isSuccess()) out=errorOf(r.getErrors()[0]); }
    else if(route=='api_system_list') { Database.UpsertResult r=Database.upsert(new List<Account>{a,b},false,AccessLevel.SYSTEM_MODE)[0]; if(!r.isSuccess()) out=errorOf(r.getErrors()[0]); }
    else if(route=='api_selector') { Database.UpsertResult r=Database.upsert(a,Account.Id,false); if(!r.isSuccess()) out=errorOf(r.getErrors()[0]); }
    else if(route=='api_selector_list') { Database.UpsertResult r=Database.upsert(new List<Account>{a,b},Account.Id,false)[0]; if(!r.isSuccess()) out=errorOf(r.getErrors()[0]); }
    else if(route=='api_user') { Database.UpsertResult r=Database.upsert(a,false,AccessLevel.USER_MODE); if(!r.isSuccess()) out=errorOf(r.getErrors()[0]); }
    else if(route=='api_system') { Database.UpsertResult r=Database.upsert(a,false,AccessLevel.SYSTEM_MODE); if(!r.isSuccess()) out=errorOf(r.getErrors()[0]); }
    else if(route=='api_existing_update') { Database.SaveResult r=Database.update(a,false); if(!r.isSuccess()) out=errorOf(r.getErrors()[0]); }
    else { Database.UpsertResult r=Database.upsert(a,false); if(!r.isSuccess()) out=errorOf(r.getErrors()[0]); }
   } catch(DmlException e) { out=e.getTypeName()+':'+e.getMessage(); }
   catch(Exception e) { out=e.getTypeName()+':'+e.getMessage(); }
   rows.add(route+'|'+out+'|'+(Limits.getDmlStatements()-statements)+'|'+(Limits.getDmlRows()-count));
  }
  return rows;
 }
 private static String errorOf(Database.Error e) { return String.valueOf(e.getStatusCode())+':'+String.join(e.getFields(),','); }
}
