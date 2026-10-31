public class PersonFollowup {
 public static List<String> observe() {
  List<String> rows=new List<String>();
  Account seed=new Account(LastName='Seed'); insert seed;
  for(String route:new List<String>{'upsert_scalar','upsert_list','update_scalar','update_list','user_null','user_list_new','user_list_existing'}) {
   for(String shape:new List<String>{'null','omitted','invalid'}) {
    Account a=new Account(LastName='Probe');
    if(route.startsWith('update') || route=='user_list_existing') a.Id=seed.Id;
    if(shape=='null') a.RecordTypeId=null;
    if(shape=='invalid') a.LastName=null;
    String out='success';
    try {
     if(route=='upsert_scalar') { Database.UpsertResult r=Database.upsert(a,false); if(!r.isSuccess()) out=String.valueOf(r.getErrors()[0].getStatusCode())+':'+String.join(r.getErrors()[0].getFields(),','); }
     else if(route=='upsert_list') { Database.UpsertResult r=Database.upsert(new List<Account>{a},false)[0]; if(!r.isSuccess()) out=String.valueOf(r.getErrors()[0].getStatusCode())+':'+String.join(r.getErrors()[0].getFields(),','); }
     else if(route=='update_scalar') { Database.SaveResult r=Database.update(a,false); if(!r.isSuccess()) out=String.valueOf(r.getErrors()[0].getStatusCode())+':'+String.join(r.getErrors()[0].getFields(),','); }
     else if(route=='update_list') { Database.SaveResult r=Database.update(new List<Account>{a},false)[0]; if(!r.isSuccess()) out=String.valueOf(r.getErrors()[0].getStatusCode())+':'+String.join(r.getErrors()[0].getFields(),','); }
     else if(route=='user_null') { a.Id=null; upsert as user a; }
     else { List<Account> records=new List<Account>{a}; upsert as user records; }
    } catch(DmlException e) { out=String.valueOf(e.getDmlStatusCode(0))+':'+String.join(e.getDmlFieldNames(0),','); } catch(Exception e) { out=e.getTypeName()+':'+e.getMessage(); }
    rows.add(route+'_'+shape+'|'+out);
   }
  }
  return rows;
 }
}
