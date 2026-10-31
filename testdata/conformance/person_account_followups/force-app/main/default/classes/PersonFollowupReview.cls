public class PersonFollowupReview {
 public static List<String> observe() {
  List<String> rows=new List<String>();
  Account seed1=new Account(LastName='Seedone'); Account seed2=new Account(LastName='Seedtwo');
  insert new List<Account>{seed1,seed2};
  for(String route:new List<String>{'name_null_only','name_null_business','name_null_first','name_omitted','user_bulk_new','user_bulk_existing','user_bulk_mixed','user_selector_new','user_selector_existing','api_upsert_user','api_upsert_system','api_upsert_selector','api_upsert_user_list','api_upsert_system_list','api_upsert_selector_list','api_update_existing','api_upsert_existing'}) {
   Account a=new Account(LastName='Probe'); Account b=new Account(LastName='Second');
   if(route.startsWith('name_')) {
    a=new Account();
    if(route!='name_omitted') a.LastName=null;
    if(route=='name_null_business') a.Name='Business';
    if(route=='name_null_first') a.FirstName='First';
   }
   if(route=='user_bulk_existing' || route=='user_selector_existing' || route=='api_update_existing' || route=='api_upsert_existing') a.Id=seed1.Id;
   if(route=='user_bulk_existing' || route=='user_bulk_mixed') b.Id=seed2.Id;
   if(route.startsWith('api_')) { a.RecordTypeId=null; b.RecordTypeId=null; }
   Integer statements=Limits.getDmlStatements(), count=Limits.getDmlRows();
   String outcome='success';
   try {
    if(route.startsWith('name_')) { Database.SaveResult r=Database.insert(a,false); if(!r.isSuccess()) outcome=errorOf(r.getErrors()[0]); }
    else if(route.startsWith('user_bulk')) { List<Account> aa=new List<Account>{a,b}; upsert as user aa; }
    else if(route.startsWith('user_selector')) { upsert as user a Account.Id; }
    else if(route=='api_upsert_user_list') { Database.UpsertResult r=Database.upsert(new List<Account>{a,b},false,AccessLevel.USER_MODE)[0]; if(!r.isSuccess()) outcome=errorOf(r.getErrors()[0]); }
    else if(route=='api_upsert_system_list') { Database.UpsertResult r=Database.upsert(new List<Account>{a,b},false,AccessLevel.SYSTEM_MODE)[0]; if(!r.isSuccess()) outcome=errorOf(r.getErrors()[0]); }
    else if(route=='api_upsert_selector_list') { Database.UpsertResult r=Database.upsert(new List<Account>{a,b},Account.Id,false)[0]; if(!r.isSuccess()) outcome=errorOf(r.getErrors()[0]); }
    else if(route=='api_upsert_user') { Database.UpsertResult r=Database.upsert(a,false,AccessLevel.USER_MODE); if(!r.isSuccess()) outcome=errorOf(r.getErrors()[0]); }
    else if(route=='api_upsert_system') { Database.UpsertResult r=Database.upsert(a,false,AccessLevel.SYSTEM_MODE); if(!r.isSuccess()) outcome=errorOf(r.getErrors()[0]); }
    else if(route=='api_upsert_selector') { Database.UpsertResult r=Database.upsert(a,Account.Id,false); if(!r.isSuccess()) outcome=errorOf(r.getErrors()[0]); }
    else if(route=='api_update_existing') { Database.SaveResult r=Database.update(a,false); if(!r.isSuccess()) outcome=errorOf(r.getErrors()[0]); }
    else { Database.UpsertResult r=Database.upsert(a,false); if(!r.isSuccess()) outcome=errorOf(r.getErrors()[0]); }
   } catch(DmlException e) { outcome=e.getTypeName()+':'+e.getDmlStatusCode(0)+':'+e.getMessage(); }
   catch(Exception e) { outcome=e.getTypeName()+':'+e.getMessage(); }
   outcome=outcome.replace(String.valueOf(seed1.Id),'<ID1>').replace(String.valueOf(seed2.Id),'<ID2>');
   rows.add(route+'|'+outcome+'|'+(Limits.getDmlStatements()-statements)+'|'+(Limits.getDmlRows()-count));
  }
  return rows;
 }
 private static String errorOf(Database.Error e) { return String.valueOf(e.getStatusCode())+':'+String.join(e.getFields(),',')+':'+e.getMessage(); }
}
