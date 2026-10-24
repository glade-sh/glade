public class TaskReferenceProbe {
 public static Map<String,Object> observe() {
  Account live=new Account(Name='live reference');
  Account removed=new Account(Name='deleted reference');
  insert new List<Account>{live,removed};
  delete removed;
  Contact wrong=new Contact(LastName='wrong reference type');
  insert wrong;
  Map<String,Id> parents=new Map<String,Id>{
   'valid'=>live.Id,
   'missing'=>(Id)(Account.SObjectType.getDescribe().getKeyPrefix()+'000000000123'),
   'deleted'=>removed.Id,
   'wrongType'=>wrong.Id
  };
  Map<String,Object> observed=new Map<String,Object>();
  for(String operation:new List<String>{'insert','update','upsertCreate','upsertUpdate'}) {
   for(String kind:new List<String>{'valid','missing','deleted','wrongType'}) {
    for(String route:new List<String>{'statement','databaseTrue','databaseFalse'}) {
     Task row=new Task(Subject='reference validation');
     if(operation=='update' || operation=='upsertUpdate') insert row;
     row.WhatId=parents.get(kind);
     String status='SUCCESS';
     List<String> fields=new List<String>();
     try {
      if(route=='statement') {
       if(operation=='insert') insert row;
       else if(operation=='update') update row;
       else upsert row;
      } else {
       Boolean allOrNone=route=='databaseTrue';
       if(operation=='upsertCreate' || operation=='upsertUpdate') {
        Database.UpsertResult result=Database.upsert(row,allOrNone);
        if(!result.isSuccess()) {
         status=String.valueOf(result.getErrors()[0].getStatusCode());
         fields=result.getErrors()[0].getFields();
        }
       } else {
        Database.SaveResult result;
        if(operation=='insert') result=Database.insert(row,allOrNone);
        else result=Database.update(row,allOrNone);
        if(!result.isSuccess()) {
         status=String.valueOf(result.getErrors()[0].getStatusCode());
         fields=result.getErrors()[0].getFields();
        }
       }
      }
     } catch(DmlException ex) {
      status=String.valueOf(ex.getDmlStatusCode(0));
      fields=ex.getDmlFieldNames(0);
     }
     observed.put(operation+'/'+kind+'/'+route,new Map<String,Object>{'status'=>status,'fields'=>fields});
    }
   }
  }
  return observed;
 }
}
