public class B3TaskDetails {
 public static String run() {
  String u=String.valueOf(Crypto.getRandomInteger());Contact c=new Contact(LastName='Synthetic'+u,Email='c'+u+'@example.invalid');insert c;List<String> out=new List<String>();
  for(Boolean supplied:new List<Boolean>{false,true}) {
   Case row=new Case(Subject='Synthetic',Status='New',Origin='Web',ContactId=c.Id,SuppliedEmail=supplied?'s'+u+'@example.invalid':null);
   Database.DMLOptions o=new Database.DMLOptions();o.EmailHeader.triggerAutoResponseEmail=true;Database.SaveResult r=Database.insert(row,o);
   List<Task> tasks=[SELECT Subject,Status,Priority,TaskSubtype,WhoId,WhatId,IsClosed,ActivityDate FROM Task WHERE WhatId=:row.Id];
   if(tasks.isEmpty())out.add('case_detail_'+supplied+'='+r.isSuccess()+',0');else{Task t=tasks[0];out.add('case_detail_'+supplied+'='+r.isSuccess()+','+tasks.size()+','+t.Subject+','+t.Status+','+t.Priority+','+t.TaskSubtype+','+(t.WhoId==c.Id)+','+(t.WhatId==row.Id)+','+t.IsClosed+','+(t.ActivityDate==Date.today()));}
  }return String.join(out,'|');
 }
}
