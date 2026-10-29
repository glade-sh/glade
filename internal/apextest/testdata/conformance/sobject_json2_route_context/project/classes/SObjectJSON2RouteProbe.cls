public class SObjectJSON2RouteProbe {
 public static String observe() {
  Savepoint sp=Database.setSavepoint();
  Account a=new Account(Name='Owned route');insert a;
  Contact c=new Contact(LastName='Owned route',AccountId=a.Id);insert c;
  Task t=new Task(Subject='Owned route',WhatId=a.Id,WhoId=c.Id);insert t;
  Pricebook2 b=new Pricebook2(Name='Owned route');insert b;
  Map<String,String> rows=new Map<String,String>();
  Map<String,Id> ids=new Map<String,Id>{'Account'=>a.Id,'Contact'=>c.Id,'Pricebook2'=>b.Id};
  for(String typ:ids.keySet()) {
   Id recordId=ids.get(typ);
   SObject row=Database.query('SELECT Id,CreatedDate,LastModifiedDate,SystemModstamp,LastViewedDate,LastReferencedDate FROM '+typ+' WHERE Id=:recordId')[0];
   rows.put(typ,String.valueOf(row.get('LastViewedDate')!=null)+'/'+String.valueOf(row.get('LastReferencedDate')!=null));
   rows.put(typ+'.sameCreated',String.valueOf(row.get('LastViewedDate')==row.get('CreatedDate'))+'/'+String.valueOf(row.get('LastReferencedDate')==row.get('CreatedDate')));
  }
  Id taskId=t.Id;
  Task task=[SELECT Id,OwnerId,Owner.Name FROM Task WHERE Id=:taskId];
  rows.put('Task.Owner',String.valueOf(task.Owner!=null));
  rows.put('Task.OwnerId',String.valueOf(task.OwnerId==UserInfo.getUserId()));
  rows.put('Task.Owner.Name',task.Owner==null?'null':task.Owner.Name==UserInfo.getName()?'current':'other');
  Database.rollback(sp);return JSON.serialize(rows);
 }
}